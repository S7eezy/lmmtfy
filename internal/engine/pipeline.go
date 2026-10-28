package engine

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"sync"

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/expr"
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
	recs     []csvio.Record
	firstRow int      // data row number of recs[0], for row()
	groups   [][]item // instead of recs when rows are combined: one per output row
	results  []rowResult
	out      []byte // formatted output rows, back to back
	err      error  // read error after the last record of this batch
	done     chan struct{}
}

// rowResult is what a worker computes for one row, before any shared state
// is touched.
type rowResult struct {
	rec        csvio.Record
	recs       []csvio.Record // every row of a batch, when rows are combined
	events     []event        // in column order
	rowErr     *cellError     // whole-row error (ragged row)
	writeErr   *cellError     // the row cannot be written as configured
	defaulted  bool
	trace      *TraceRow // how the traced column was filled
	cells      []string  // the output row when apply numbers it (sequences)
	start, end int       // formatted row in batch.out
}

type eventKind int

const (
	eventWarning   eventKind = iota // value altered, written
	eventDefaulted                  // error replaced by the default (on_error: default)
	eventReject                     // error rejecting the row (on_error: skip_row)
	eventFail                       // error failing the file (on_error: fail_file)
)

type event struct {
	col  *active
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

	// Reader. send queues a batch for the workers and for apply; false
	// when the pipeline stopped.
	send := func(b *batch) bool {
		select {
		case ordered <- b:
		case <-stop:
			return false
		}
		select {
		case work <- b:
		case <-stop:
			close(b.done) // queued but never computed: unblock the drain
			return false
		}
		return true
	}
	wg.Go(func() {
		defer close(work)
		defer close(ordered)
		if r.plan.group != nil {
			r.readBatches(reader, first, send)
			return
		}
		var pending []csvio.Record
		if first != nil {
			pending = append(pending, *first)
		}
		read := len(pending)
		limit := r.job.MaxRows
		rows := 0
		for {
			b := &batch{recs: pending, firstRow: rows + 1, done: make(chan struct{})}
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
			rows += len(b.recs)
			last := len(b.recs) < batchSize || limit > 0 && read >= limit
			if !send(b) || last {
				return
			}
		}
	})

	// Workers.
	for range workers {
		wg.Go(func() {
			w := r.newWorker()
			for b := range work {
				r.compute(b, w)
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

// worker is the scratch state of one worker goroutine.
type worker struct {
	f      *csvio.Formatter // its own: encoders are stateful
	out    []string         // the output row
	env    expr.Env         // values formulas read, by column id
	failed []bool           // columns whose value is missing after an error, by id
	dirty  bool             // failed has entries set
	aggs   []expr.Value     // a batch's aggregates, reused
}

func (r *run) newWorker() *worker {
	f, _ := csvio.NewFormatter(r.wopts) // options already checked by openWriter
	n := len(r.plan.nodes)
	return &worker{
		f:      f,
		out:    make([]string, len(r.cols)),
		env:    expr.Env{Columns: make([]expr.Value, n), Aggregates: r.aggregates, Start: r.start},
		failed: make([]bool, n),
	}
}

// compute converts and formats the rows of a batch. It runs on worker
// goroutines: it only reads the plan and writes to the batch.
func (r *run) compute(b *batch, w *worker) {
	if b.groups != nil {
		b.results = make([]rowResult, len(b.groups))
		for i, items := range b.groups {
			r.computeGroup(&b.results[i], b, w, items)
		}
		return
	}
	b.results = make([]rowResult, len(b.recs))
	for i, rec := range b.recs {
		res := &b.results[i]
		res.rec = rec
		w.env.Row = b.firstRow + i
		r.computeRow(res, b, w)
	}
}

func (r *run) computeRow(res *rowResult, b *batch, w *worker) {
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

	w.reset()
	rejected := false
	for _, c := range r.order {
		// The hot path, inline: a plain cell converting without error.
		// Anything else goes through fill (which converts it again).
		if !c.computes() && c != r.traced {
			if v, empty, err := c.value(fields, p); err == nil {
				if c.pos >= 0 {
					w.out[c.pos] = v
				}
				if !c.referenced || r.store(c, w, v, empty) == nil {
					continue
				}
			}
		}
		if r.fill(c, fields, w, res, &rejected) {
			return
		}
	}
	if !rejected {
		r.finish(res, b, w)
	}
}

// fill computes one column of a row: its output cell and, when formulas
// read it, its value. Errors follow the column's strategy; stop is true
// when the file fails (apply stops there).
func (r *run) fill(c *active, fields []string, w *worker, res *rowResult, rejected *bool) (stop bool) {
	p := r.plan
	var (
		v     string
		empty bool
		err   error
	)
	switch {
	case !c.computes():
		v, empty, err = c.value(fields, p)
	case c.seq != nil:
		// Numbered when the row is written, in order (apply).
		w.out[c.pos] = ""
		return false
	case *rejected && r.readsFailed(c, w):
		// It reads a column whose error already rejects the row: one
		// error is enough.
		r.fail(c, w)
		if c == r.traced {
			res.trace = &TraceRow{Error: "not computed: a column it reads has an error"}
		}
		return false
	case c.cases != nil:
		v, empty, err = c.cased(p, &w.env)
	default:
		v, empty, err = c.computed(fields, p, &w.env)
	}
	if c == r.traced {
		res.trace = r.traceRow(c, fields, w, v, err)
	}
	if err != nil {
		var warn *types.Warning
		if errors.As(err, &warn) {
			res.events = append(res.events, event{c, eventWarning, warn.Msg})
			err = nil
		}
	}
	if err == nil {
		if c.pos >= 0 {
			w.out[c.pos] = v
		}
		if !c.referenced {
			return false
		}
		if err = r.store(c, w, v, empty); err == nil {
			return false
		}
	}
	return r.failCell(c, w, res, err, rejected)
}

// failCell applies a column's error strategy to an error.
func (r *run) failCell(c *active, w *worker, res *rowResult, err error, rejected *bool) (stop bool) {
	switch c.onError {
	case "fail_file":
		// Columns after this one are not evaluated: apply stops here.
		res.events = append(res.events, event{c, eventFail, err.Error()})
		return true
	case "default":
		res.defaulted = true
		res.events = append(res.events, event{c, eventDefaulted, err.Error()})
		out, empty := c.fallback(r.plan), c.fallbackEmpty()
		if c.pos >= 0 {
			w.out[c.pos] = out
		}
		if c.referenced && r.store(c, w, out, empty) != nil {
			r.fail(c, w)
		}
	default:
		*rejected = true
		r.fail(c, w)
		res.events = append(res.events, event{c, eventReject, err.Error()})
	}
	return false
}

// finish formats a computed row, or leaves it to apply when it numbers
// rows.
func (r *run) finish(res *rowResult, b *batch, w *worker) {
	if r.numbered {
		res.cells = slices.Clone(w.out) // formatted by apply, once numbered
		return
	}
	res.start = len(b.out)
	formatted, err := w.f.Append(b.out, w.out)
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

// store keeps the value of a column that formulas read.
func (r *run) store(c *active, w *worker, out string, empty bool) error {
	v, err := c.toValue(out, empty)
	w.env.Columns[c.id] = v
	return err
}

func (r *run) readsFailed(c *active, w *worker) bool {
	return slices.ContainsFunc(c.refs, func(id int) bool { return w.failed[id] })
}

// reset forgets the failed columns of the previous row.
func (w *worker) reset() {
	if w.dirty {
		clear(w.failed)
		w.dirty = false
	}
}

// fail marks a column as having no value in this row.
func (r *run) fail(c *active, w *worker) {
	w.failed[c.id] = true
	w.env.Columns[c.id] = expr.Value{}
	w.dirty = true
}
