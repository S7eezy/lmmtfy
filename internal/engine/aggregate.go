package engine

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/expr"
	"github.com/S7eezy/lmmtfy/internal/report"
	"github.com/S7eezy/lmmtfy/internal/types"
)

// Aggregates (lowest, total, ...) need the whole file before the first row
// is converted: a first pass reads the input once, computes the columns
// their arguments read and folds the values. The conversion then reads the
// input again, from the start.

// rereadable makes sure the input can be read twice: a file or a buffer
// can (io.ReaderAt); anything else (a pipe) is copied to a temporary file
// first.
func (r *run) rereadable() (cleanup func(), err error) {
	if _, ok := r.job.Input.(io.ReaderAt); ok {
		if s, ok := r.job.Input.(io.Seeker); ok {
			r.inputStart, err = s.Seek(0, io.SeekCurrent)
			return func() {}, err
		}
	}
	f, err := os.CreateTemp("", "lmmtfy-input-*.csv")
	if err != nil {
		return nil, err
	}
	cleanup = func() {
		f.Close()
		os.Remove(f.Name())
	}
	if _, err := io.Copy(f, r.job.Input); err != nil {
		cleanup()
		return nil, fmt.Errorf("copying the input to read it twice (aggregates): %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, err
	}
	r.job.Input = f
	return cleanup, nil
}

// firstPass computes every aggregate. It reads the input from where the
// conversion starts, through its own reader.
func (r *run) firstPass() error {
	p := r.plan
	// A section reader reads at offsets, without moving the input that the
	// conversion reads.
	start := r.inputStart
	reader, err := csvio.NewReader(io.NewSectionReader(r.job.Input.(io.ReaderAt), start, math.MaxInt64-start), p.reader)
	if err != nil {
		return err
	}
	skip := 0
	if p.header {
		skip = 1 + p.skipAfterHeader
	}

	var order []*active
	for _, a := range r.order {
		if p.passCols[a.id] {
			order = append(order, a)
		}
	}
	accs := make([]*expr.Accumulator, len(p.aggregates))
	leftOut := make([]int, len(p.aggregates))
	for i, a := range p.aggregates {
		accs[i] = expr.NewAccumulator(a)
	}
	w := r.newWorker()
	limit := r.job.MaxRows
	if r.job.AggregateAll {
		limit = 0
	}

	rows := 0
	for {
		rec, err := readRecord(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if skip > 0 {
			skip--
			continue
		}
		if limit > 0 && rows >= limit {
			break
		}
		rows++
		if err := r.job.Context.Err(); err != nil {
			return err
		}

		fields := rec.Fields
		if n := len(fields); n != r.width {
			switch {
			case n < r.width && (p.ragged == "pad" || p.ragged == "lenient"):
				fields = append(slices.Clone(fields), make([]string, r.width-n)...)
			case n > r.width && (p.ragged == "truncate" || p.ragged == "lenient"):
			default:
				for i := range leftOut {
					leftOut[i]++ // the conversion rejects or fails on this row
				}
				continue
			}
		}

		clear(w.failed)
		w.env.Row = rows
		for _, c := range order {
			var (
				out   string
				empty bool
				err   error
			)
			switch {
			case !c.computes():
				out, empty, err = c.value(fields, p)
			case slices.ContainsFunc(c.refs, func(id int) bool { return w.failed[id] }):
				err = errors.New("a column it reads failed")
			case c.cases != nil:
				out, empty, err = c.cased(p, &w.env)
			default:
				out, empty, err = c.computed(fields, p, &w.env)
			}
			if err == nil || errors.As(err, new(*types.Warning)) {
				err = r.store(c, w, out, empty)
			}
			if err != nil {
				r.fail(c, w)
			}
		}

		done := true
		for i, a := range p.aggregates {
			acc := accs[i]
			if acc.Done() {
				continue
			}
			done = false
			if a.Arg == nil {
				acc.Add(expr.Bool(true))
				continue
			}
			if slices.ContainsFunc(a.Columns(), func(id int) bool { return w.failed[id] }) {
				leftOut[i]++
				continue
			}
			v, err := a.Arg.Eval(&w.env)
			if err != nil {
				leftOut[i]++
				continue
			}
			acc.Add(v)
		}
		if done {
			break // only first() values, all found
		}
	}

	r.aggregates = make([]expr.Value, len(accs))
	log := r.job.Log
	for i, acc := range accs {
		r.aggregates[i] = acc.Result()
		name := r.aggregateName(i)
		if leftOut[i] > 0 {
			what := "values could not be read and were left out"
			if leftOut[i] == 1 {
				what = "value could not be read and was left out"
			}
			log.Warn(report.Event{Message: fmt.Sprintf("%s: %d %s", name, leftOut[i], what)})
		}
		if log.Enabled(report.LevelInfo) {
			log.Info(report.Event{Message: fmt.Sprintf("%s = %s", name, display(r.aggregates[i]))})
		}
	}
	return nil
}

// aggregateName names an aggregate in messages, by the column it is for.
func (r *run) aggregateName(slot int) string {
	for _, c := range r.plan.nodes {
		for _, a := range c.aggs {
			if a.Slot == slot {
				return fmt.Sprintf("%s(…) in %s", a.Kind, c.name)
			}
		}
	}
	return "aggregate"
}

func display(v expr.Value) string {
	if v.IsEmpty() {
		return "empty (no values)"
	}
	return v.Rounded(10).Text()
}

// defaults computes this file's default formulas, once aggregates are
// known, and puts them where defaults are used.
func (r *run) defaults() error {
	env := expr.Env{Aggregates: r.aggregates, Start: r.start}
	for _, a := range r.order {
		if a.defaultFormula == nil {
			continue
		}
		v, err := a.defaultFormula.Eval(&env)
		if err != nil {
			return &FileError{Column: a.name, Reason: "default formula: " + err.Error()}
		}
		out := ""
		if !v.IsEmpty() {
			if out, err = a.format.Convert(expr.Convert(v, a.typ).Canonical()); err != nil && !errors.As(err, new(*types.Warning)) {
				return &FileError{Column: a.name, Reason: fmt.Sprintf("default formula gives %q: %v", v.Text(), err)}
			}
		}
		a.defaultOut = &out
		if a.constDefault {
			a.constOut, a.constEmpty = out, out == ""
		}
	}
	return nil
}
