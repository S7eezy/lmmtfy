package engine

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/report"
)

// Duplicate rows: output rows with the same values in the key columns,
// as written. The ordered apply stage decides which one is kept, so the
// result does not depend on the number of workers.
//
// keep: first keeps a set of the keys written. keep: last converts the
// file twice: a dry run, writing nothing, finds the line of the last row
// of each key that converts; the real run writes only those.

type dupPlan struct {
	cols []int // ids of the key columns
	last bool
}

func (p *Plan) compileDuplicates(issues *config.Issues) {
	d := p.Mapping.Duplicates
	if d == nil {
		return
	}
	dp := &dupPlan{last: d.Keep == "last"}
	for _, name := range d.Key {
		for _, c := range p.cols {
			if c.name != name {
				continue
			}
			if c.seq != nil {
				issues.Errorf("duplicates.key", "[%s] is a row number: every row has its own", name)
			}
			dp.cols = append(dp.cols, c.id)
		}
	}
	p.dups = dp
}

// dupState is what a run remembers of the keys.
type dupState struct {
	pos      []int // output positions of the key columns present in this input
	seen     map[string]struct{}
	lastLine map[string]int // keep: last, from the dry run
	dry      bool           // this run only records lastLine
}

func (r *run) prepareDuplicates() {
	d := r.plan.dups
	if d == nil {
		return
	}
	for _, id := range d.cols {
		for _, a := range r.cols {
			if a.id == id {
				r.dup.pos = append(r.dup.pos, a.pos)
			}
		}
	}
	if !d.last {
		r.dup.seen = map[string]struct{}{}
	}
}

// dupKey is the key of an output row.
func (r *run) dupKey(out []string) string {
	var b strings.Builder
	for i, pos := range r.dup.pos {
		if i > 0 {
			b.WriteByte(0)
		}
		b.WriteString(out[pos])
	}
	return b.String()
}

// duplicate reports whether apply drops a row that converted; in the dry
// run, it records the row and drops it (nothing is written).
func (r *run) duplicate(res *rowResult) bool {
	d := r.plan.dups
	switch {
	case d == nil:
		return false
	case r.dup.dry:
		r.dup.lastLine[res.key] = res.rec.Line
		return true
	case d.last:
		line, ok := r.dup.lastLine[res.key]
		return ok && line != res.rec.Line
	}
	if _, ok := r.dup.seen[res.key]; ok {
		return true
	}
	r.dup.seen[res.key] = struct{}{}
	return false
}

// runTwice runs keep: last: a dry run to find the last row of each key,
// then the conversion. Both read the input from where the job starts.
func (r *run) runTwice() error {
	cleanup, err := r.rereadable()
	if err != nil {
		return err
	}
	defer cleanup()
	src, start := r.job.Input.(io.ReaderAt), r.inputStart
	section := func() io.Reader { return io.NewSectionReader(src, start, math.MaxInt64-start) }

	dry := &run{plan: r.plan, job: r.job, sum: &report.Summary{}, start: r.start}
	dry.job.Input, dry.job.Output, dry.job.Rejects = section(), io.Discard, nil
	dry.job.Log, dry.job.Trace, dry.job.Sources = report.Discard, nil, nil
	dry.dup.dry, dry.dup.lastLine = true, map[string]int{}
	// A failure stops the conversion at the same row: the real run
	// reports it.
	if err := dry.run(); errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	r.dup.lastLine = dry.dup.lastLine
	r.job.Input, r.inputStart = section(), 0
	return r.run()
}
