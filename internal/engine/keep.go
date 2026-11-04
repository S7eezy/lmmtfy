package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/expr"
)

// keep_rows_when drops the rows where a condition is false or empty,
// without errors. The condition is evaluated as early as possible: the
// columns it reads come first in the evaluation order, then the condition,
// then the other columns, so a dropped row has no errors from columns it
// never needed.

const keepPath = "keep_rows_when"

// compileKeep compiles keep_rows_when. It runs once every column is
// compiled and its aggregates numbered, before columns are ordered.
func (p *Plan) compileKeep(issues *config.Issues) {
	src := strings.TrimSpace(p.Mapping.KeepRowsWhen)
	if src == "" {
		return
	}
	prog, err := expr.Compile(src, noValue{scope{p: p}})
	if err == nil && prog.Type != expr.Boolean && prog.Type != expr.Empty {
		err = errNotCondition(prog.Type)
	}
	if err != nil {
		issues.Errorf(keepPath, "%v", err)
		return
	}
	if prog.Volatile {
		issues.Warnf(keepPath, "today() and now() give a different result on every run")
	}
	ok := true
	if len(prog.Aggregates) > 0 {
		issues.Errorf(keepPath, "aggregates cover the rows that are kept, so the condition cannot use them")
		ok = false
	}
	needs := p.needsAggregates()
	for _, id := range prog.Columns {
		n := p.nodes[id]
		switch {
		case n.seq != nil:
			issues.Errorf(keepPath, "[%s] is numbered when rows are written: the condition cannot read it (row() is the input row number)", n.name)
			ok = false
		case needs(n):
			issues.Errorf(keepPath, "[%s] is computed from an aggregate, and aggregates cover the rows that are kept: the condition cannot read it", n.name)
			ok = false
		case p.group != nil && !n.helper:
			issues.Errorf(keepPath, "when rows are combined, the condition is checked for each input row: it reads helpers, and [%s] is an output column", n.name)
			ok = false
		}
	}
	if ok {
		p.keep = prog
	}
}

// keepFirst moves the columns the condition reads, directly or through
// other formulas, to the front of the evaluation order, keeping their
// relative order (dependencies come first already).
func (p *Plan) keepFirst() {
	if p.keep == nil {
		return
	}
	need := map[int]bool{}
	var add func(id int)
	add = func(id int) {
		if need[id] {
			return
		}
		need[id] = true
		p.nodes[id].forKeep = true
		for _, r := range p.nodes[id].refs {
			add(r)
		}
	}
	for _, id := range p.keep.Columns {
		add(id)
	}
	// A stable partition: the condition's columns, then the others.
	before := slices.DeleteFunc(slices.Clone(p.order), func(id int) bool { return !need[id] })
	after := slices.DeleteFunc(slices.Clone(p.order), func(id int) bool { return need[id] })
	p.order = append(before, after...)
	for id := range need {
		p.passCols[id] = true // the first pass skips dropped rows too
	}
}

func errNotCondition(t expr.Type) error {
	return fmt.Errorf("a condition is true or false, but this gives a %s value: compare it, like [column] > 0", t)
}

// needsAggregates reports whether a column's value depends on an
// aggregate, outside its default formula.
func (p *Plan) needsAggregates() func(*column) bool {
	memo := map[int]bool{}
	var needs func(c *column) bool
	needs = func(c *column) bool {
		if v, ok := memo[c.id]; ok {
			return v
		}
		memo[c.id] = false // cycles are reported by orderColumns
		v := len(c.aggs) > c.defaultAggs
		for _, id := range c.refs {
			v = v || needs(p.nodes[id])
		}
		memo[c.id] = v
		return v
	}
	return needs
}

// keeps decides whether a row is kept, once the columns the condition
// reads are computed. A row whose condition cannot be decided (a column
// it reads has an error) is kept, with that error. A dropped row loses
// the events of the columns already computed.
func (r *run) keeps(w *worker) (keep bool, err error) {
	p := r.plan
	if slices.ContainsFunc(p.keep.Columns, func(id int) bool { return w.failed[id] }) {
		return true, nil
	}
	v, err := p.keep.Eval(&w.env)
	if err != nil {
		return true, err
	}
	return v.IsTrue(), nil
}
