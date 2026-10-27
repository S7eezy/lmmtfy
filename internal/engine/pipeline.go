package engine

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"sync"

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/types"
)

// The pipeline has three stages:
//
//	reader ──batches──▶ workers (×N) ──▶ apply, in input order ──▶ writer
//	   └────────── same batches, in order ──────────┘
//
// The reader cuts the input into batches and sends each one twice: to the
// workers, who convert and format its rows in parallel, and to the ordered
// queue, from which apply takes batches in input order, waiting for each to
// be done. Only apply touches counters, logs, the reject file and the
// output, so results do not depend on the number of workers.
//
// Memory is bounded: the ordered queue holds at most 4 batches per worker.

// batchSize is the number of rows per batch. A variable so tests can use
// tiny batches and exercise batch boundaries.
var batchSize = 1024

type batch struct {
	recs    []csvio.Record
	results []rowResult
	out     []byte // formatted output rows, back to back
	err     error  // read error after the last record of this batch
	done    chan struct{}
}

// rowResult is what a worker computes for one row, before any shared state
// is touched.
type rowResult struct {
	rec        csvio.Record
	events     []event    // in column order
	rowErr     *cellError // whole-row error (ragged row)
	writeErr   *cellError // the row cannot be written as configured
	defaulted  bool
	start, end int // formatted row in batch.out
}

type eventKind int

const (
	eventWarning   eventKind = iota // value altered, written
	eventDefaulted                  // error replaced by the default (on_error: default)
	eventReject                     // error rejecting the row (on_error: skip_row)
	eventFail                       // error failing the file (on_error: fail_file)
)

type event struct {
	col  int
	kind eventKind
	msg  string
}

func (r *run) pipeline(reader *csvio.Reader, first *csvio.Record) error {
	workers := r.job.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}

	work := make(chan *batch, workers)
	ordered := make(chan *batch, workers*4)
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Reader.
	wg.Go(func() {
		defer close(work)
		defer close(ordered)
		var pending []csvio.Record
		if first != nil {
			pending = append(pending, *first)
		}
		read := len(pending)
		limit := r.job.MaxRows
		for {
			b := &batch{recs: pending, done: make(chan struct{})}
			pending = nil
			for len(b.recs) < batchSize {
				if limit > 0 && read >= limit {
					break
				}
				read++
				rec, err := readRecord(reader)
				if err != nil {
					if !errors.Is(err, io.EOF) {
						b.err = err
					}
					break
				}
				b.recs = append(b.recs, rec)
			}
			last := len(b.recs) < batchSize || limit > 0 && read >= limit
			select {
			case ordered <- b:
			case <-stop:
				return
			}
			select {
			case work <- b:
			case <-stop:
				close(b.done) // queued but never computed: unblock the drain
				return
			}
			if last {
				return
			}
		}
	})

	// Workers. Each has its own formatter (encoders are stateful) and its own
	// scratch row.
	for range workers {
		wg.Go(func() {
			f, _ := csvio.NewFormatter(r.wopts) // options already checked by openWriter
			scratch := make([]string, len(r.cols))
			for b := range work {
				r.compute(b, f, scratch)
				close(b.done)
			}
		})
	}

	// Apply, in order, on this goroutine.
	err := func() error {
		for b := range ordered {
			<-b.done
			if err := r.job.Context.Err(); err != nil {
				return err
			}
			for i := range b.results {
				res := &b.results[i]
				if err := r.apply(res, b.out[res.start:res.end]); err != nil {
					return err
				}
			}
			if b.err != nil {
				return b.err
			}
		}
		return nil
	}()

	close(stop)
	// Drain what the reader already queued, so no goroutine stays blocked.
	for b := range ordered {
		<-b.done
	}
	wg.Wait()
	return err
}

// compute converts and formats the rows of a batch. It runs on worker
// goroutines: it only reads the plan and writes to the batch.
func (r *run) compute(b *batch, f *csvio.Formatter, out []string) {
	b.results = make([]rowResult, len(b.recs))
	for i, rec := range b.recs {
		res := &b.results[i]
		res.rec = rec
		r.computeRow(res, b, f, out)
	}
}

func (r *run) computeRow(res *rowResult, b *batch, f *csvio.Formatter, out []string) {
	p := r.plan
	fields := res.rec.Fields
	if n := len(fields); n != r.width {
		switch {
		case n < r.width && (p.ragged == "pad" || p.ragged == "lenient"):
			fields = append(slices.Clone(fields), make([]string, r.width-n)...)
		case n > r.width && (p.ragged == "truncate" || p.ragged == "lenient"):
		default:
			res.rowErr = &cellError{"", fmt.Sprintf("row has %d fields, expected %d", n, r.width)}
			return
		}
	}

	rejected := false
	for i, c := range r.cols {
		v, err := c.value(fields, p)
		if err == nil {
			out[i] = v
			continue
		}
		var warn *types.Warning
		if errors.As(err, &warn) {
			out[i] = v
			res.events = append(res.events, event{i, eventWarning, warn.Msg})
			continue
		}
		switch c.onError {
		case "fail_file":
			// Columns after this one are not evaluated: apply stops here.
			res.events = append(res.events, event{i, eventFail, err.Error()})
			return
		case "default":
			out[i] = c.fallback(p)
			res.defaulted = true
			res.events = append(res.events, event{i, eventDefaulted, err.Error()})
		default:
			rejected = true
			res.events = append(res.events, event{i, eventReject, err.Error()})
		}
	}
	if rejected {
		return
	}

	res.start = len(b.out)
	formatted, err := f.Append(b.out, out)
	if err != nil {
		var re *csvio.RowError
		if !errors.As(err, &re) {
			re = &csvio.RowError{Field: -1, Msg: err.Error()}
		}
		column := ""
		if re.Field >= 0 {
			column = r.cols[re.Field].name
		}
		res.writeErr = &cellError{column, re.Msg}
		res.end = res.start
		return
	}
	b.out = formatted
	res.end = len(b.out)
}
