package engine

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/expr"
)

// Combining rows: rows sharing the values of group.by form a batch, which
// becomes one output row. Helpers are computed for each row of the batch;
// pattern columns (price_{n}) read one row each, the nth; other columns
// read the key, the batch's aggregates, or other output columns.
//
// The reader builds batches, so workers convert whole batches in parallel
// and apply keeps them in order, as for rows.

type groupPlan struct {
	by       []string
	orderBy  []int // helper ids
	sorted   bool
	overflow string
	slots    int // number of slots of the patterns, 0 without patterns
}

// compileGroup checks what each column may read when rows are combined.
func (p *Plan) compileGroup(issues *config.Issues) {
	g := p.group
	for _, name := range p.Mapping.Group.OrderBy {
		for _, c := range p.nodes {
			if c.helper && c.name == name {
				g.orderBy = append(g.orderBy, c.id)
			}
		}
	}
	for _, c := range p.nodes {
		for _, id := range c.refs {
			n := p.nodes[id]
			switch {
			case c.helper && !n.helper:
				issues.Errorf(c.formulaPath(), "helpers are computed for each row of a batch, before output columns: [%s] is an output column", n.name)
			case !c.helper && c.slot < 0 && n.helper:
				issues.Errorf(c.formulaPath(), "[%s] has one value per row of a batch: use first([%s]), last(…), lowest(…), total(…) or a pattern column", n.name, n.name)
			}
		}
		if c.helper && len(c.aggs) > 0 {
			issues.Errorf(c.aggregatePath(), "helpers are computed for each row of a batch, before the batch's aggregates")
			continue
		}
		for i, a := range c.aggs {
			if i < c.defaultAggs {
				issues.Errorf(c.path+".default_formula", "when rows are combined, aggregates cover each batch: a default formula cannot use them")
				break
			}
			for _, id := range a.Columns() {
				if n := p.nodes[id]; !n.helper {
					issues.Errorf(c.aggregatePath(), "%s([%s]): aggregates of a batch read helpers, computed for each of its rows", a.Kind, n.name)
				}
			}
		}
	}
}

// item is one input row of a batch.
type item struct {
	rec csvio.Record
	row int // data row number, for row()
}

// keyOf returns the batch key of a row: the values of the group.by
// columns, after trimming and null tokens.
func (r *run) keyOf(fields []string) string {
	var b strings.Builder
	for i, idx := range r.byIdx {
		if i > 0 {
			b.WriteByte(0)
		}
		if idx < len(fields) {
			v := strings.TrimSpace(fields[idx])
			if !r.plan.nulls[v] {
				b.WriteString(v)
			}
		}
	}
	return b.String()
}

func showKey(key string) string { return fmt.Sprintf("%q", strings.ReplaceAll(key, "\x00", ", ")) }

// computeGroup converts one batch into one output row.
func (r *run) computeGroup(res *rowResult, b *batch, w *worker, items []item) {
	p := r.plan
	g := p.group
	res.rec = items[0].rec
	res.recs = make([]csvio.Record, len(items))
	for i, it := range items {
		res.recs[i] = it.rec
	}

	fields := make([][]string, len(items))
	for i, it := range items {
		f, msg := r.fields(it.rec.Fields)
		if msg != "" {
			res.rowErr = &cellError{"", fmt.Sprintf("line %d: %s", it.rec.Line, msg)}
			return
		}
		fields[i] = f
	}

	// Helpers, for each row.
	w.reset()
	rejected := false
	vals := make([][]expr.Value, len(items))
	for i := range items {
		w.env.Row = items[i].row
		for _, c := range r.order {
			if c.helper && r.fill(c, fields[i], w, res, &rejected) {
				return
			}
		}
		if rejected {
			return
		}
		vals[i] = slices.Clone(w.env.Columns)
	}

	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	if len(g.orderBy) > 0 {
		slices.SortStableFunc(order, func(a, b int) int {
			for _, id := range g.orderBy {
				if c := expr.Compare(vals[a][id], vals[b][id]); c != 0 {
					return c
				}
			}
			return 0
		})
	}

	// The batch's aggregates, over all its rows.
	w.env.Aggregates = w.aggs[:0]
	for _, a := range p.aggregates {
		acc := expr.NewAccumulator(a)
		for _, i := range order {
			if a.Arg == nil {
				acc.Add(expr.Bool(true))
				continue
			}
			copy(w.env.Columns, vals[i])
			w.env.Row = items[i].row
			v, err := a.Arg.Eval(&w.env)
			if err != nil {
				continue // the helper's own error was reported
			}
			acc.Add(v)
		}
		w.env.Aggregates = append(w.env.Aggregates, acc.Result())
	}
	w.aggs = w.env.Aggregates

	used := len(order)
	if g.slots > 0 && used > g.slots {
		msg := fmt.Sprintf("the batch %s has %d rows, more than the %d slots", showKey(r.keyOf(fields[0])), used, g.slots)
		if g.overflow != "drop_extra" {
			res.rowErr = &cellError{"", msg + " (group.overflow: error)"}
			return
		}
		res.events = append(res.events, event{r.firstSlot(), eventWarning, msg + ": the extra rows are dropped"})
		used = g.slots
	}

	// Output columns: slots read their row, others the batch's first row.
	first := order[0]
	for _, c := range r.order {
		if c.helper {
			continue
		}
		at := first
		if c.slot >= 0 {
			if c.slot >= used {
				if r.emptySlot(c, w, res, &rejected, used) {
					return
				}
				continue
			}
			at = order[c.slot]
		}
		for _, h := range r.helpers {
			w.env.Columns[h] = vals[at][h]
		}
		w.env.Row = items[at].row
		if r.fill(c, fields[at], w, res, &rejected) {
			return
		}
	}
	if !rejected {
		r.finish(res, b, w)
	}
}

// firstSlot is the first pattern column, which batch-wide warnings are
// counted on.
func (r *run) firstSlot() *active {
	for _, c := range r.cols {
		if c.slot >= 0 {
			return c
		}
	}
	return r.cols[0]
}

// emptySlot fills a pattern column the batch has no row for: its default,
// or empty.
func (r *run) emptySlot(c *active, w *worker, res *rowResult, rejected *bool, rows int) (stop bool) {
	out, empty := c.fallback(r.plan), c.fallbackEmpty()
	if empty && c.required {
		return r.failCell(c, w, res, fmt.Errorf("required value is empty: the batch has %d rows, no row for this slot", rows), rejected)
	}
	w.out[c.pos] = out
	if c.referenced {
		if err := r.store(c, w, out, empty); err != nil {
			return r.failCell(c, w, res, err, rejected)
		}
	}
	return false
}

// fields applies input.ragged_rows to a row; msg explains a row that
// cannot be used.
func (r *run) fields(fields []string) ([]string, string) {
	p := r.plan
	n := len(fields)
	switch {
	case n == r.width:
	case n < r.width && (p.ragged == "pad" || p.ragged == "lenient"):
		fields = append(slices.Clone(fields), make([]string, r.width-n)...)
	case n > r.width && (p.ragged == "truncate" || p.ragged == "lenient"):
	default:
		return nil, fmt.Sprintf("row has %d fields, expected %d", n, r.width)
	}
	return fields, ""
}

// previewReadLimit bounds what a preview reads when batches are not
// sorted, since a batch may then end anywhere in the file.
const previewReadLimit = 10000

// readBatches is the reader of combined rows: it cuts the input into
// batches and sends them, batchSize at a time.
func (r *run) readBatches(reader *csvio.Reader, first *csvio.Record, send func(*batch) bool) {
	g := r.plan.group
	limit := r.job.MaxRows // output rows, for previews
	b := &batch{groups: [][]item{}, done: make(chan struct{})}
	emitted := 0
	add := func(items []item) bool {
		b.groups = append(b.groups, items)
		emitted++
		if len(b.groups) < batchSize {
			return true
		}
		ok := send(b)
		b = &batch{groups: [][]item{}, done: make(chan struct{})}
		return ok
	}

	var (
		current []item
		key     string
		seen    = map[string]int{} // finished batches → their first line (sorted input)
		keys    []string           // batches in order of first appearance (unsorted input)
		pending = map[string][]item{}
		row     int
		full    bool // the preview has its rows
	)
	for {
		if limit > 0 && (g.sorted && emitted >= limit || !g.sorted && row >= previewReadLimit) {
			full = true
			break
		}
		var rec csvio.Record
		if first != nil {
			rec, first = *first, nil
		} else {
			var err error
			if rec, err = readRecord(reader); err != nil {
				if !errors.Is(err, io.EOF) {
					b.err = err
				}
				break
			}
		}
		row++
		k := r.keyOf(rec.Fields)
		it := item{rec, row}
		if !g.sorted {
			if _, ok := pending[k]; !ok {
				keys = append(keys, k)
			}
			pending[k] = append(pending[k], it)
			continue
		}
		if current != nil && k != key {
			seen[key] = current[0].rec.Line
			if !add(current) {
				return
			}
			current = nil
		}
		if line, ok := seen[k]; ok {
			b.err = &FileError{Line: rec.Line, Reason: fmt.Sprintf("the batch %s starts again (first seen at line %d): the file is not sorted by %s; set group.sorted: false",
				showKey(k), line, strings.Join(g.by, ", "))}
			current = nil
			break
		}
		current = append(current, it)
		key = k
	}
	if b.err == nil {
		if current != nil && !full {
			if !add(current) {
				return
			}
		}
		for _, k := range keys {
			if limit > 0 && emitted >= limit {
				break
			}
			if !add(pending[k]) {
				return
			}
		}
	}
	send(b)
}
