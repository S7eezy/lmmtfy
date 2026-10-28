package engine

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/expr"
	"github.com/S7eezy/lmmtfy/internal/types"
)

// Formulas work on typed values. Columns without formulas convert their
// source straight into its output form, as before; formulas exchange
// values with converters through canonical text (expr.Value.Canonical):
//
//	source ──parse──▶ value ──formula──▶ result ──format──▶ output
//	                                                        │
//	         [column] in other formulas ◀──back─────────────┘
//
// A formula reading [column] sees the column's value as written: rounded,
// cased, defaulted. Values that must not be altered go through a helper.

// canonicalColumn is the column of the same type writing canonical text,
// with no constraints.
func canonicalColumn(tc config.TargetColumn) config.TargetColumn {
	c := config.TargetColumn{Name: tc.Name, Type: tc.Type}
	switch tc.Type {
	case "date":
		c.Format = expr.DateFormat
	case "datetime":
		c.Format = expr.DatetimeFormat
		c.Timezone = tc.Timezone
	case "time":
		c.Format = expr.TimeFormat
	}
	return c
}

// canonicalRule reads canonical text.
func canonicalRule(tc config.TargetColumn) *config.Rule {
	r := &config.Rule{}
	switch tc.Type {
	case "boolean":
		r.TrueValues, r.FalseValues = []config.Scalar{"true"}, []config.Scalar{"false"}
	case "date":
		r.Formats = []string{expr.DateFormat}
	case "datetime":
		r.Formats = []string{expr.DatetimeFormat}
	case "time":
		r.Formats = []string{expr.TimeFormat}
	}
	return r
}

// outputRule reads the column's output representation.
func outputRule(tc config.TargetColumn) *config.Rule {
	r := &config.Rule{}
	switch tc.Type {
	case "integer":
		r.ThousandsSep = tc.ThousandsSep
	case "decimal":
		r.DecimalSep, r.ThousandsSep = tc.DecimalSep, tc.ThousandsSep
	case "boolean":
		r.TrueValues, r.FalseValues = []config.Scalar{"true"}, []config.Scalar{"false"}
		if tc.TrueValue != nil {
			r.TrueValues = []config.Scalar{*tc.TrueValue}
		}
		if tc.FalseValue != nil {
			r.FalseValues = []config.Scalar{*tc.FalseValue}
		}
	case "date", "datetime", "time":
		r.Formats = []string{cmp.Or(tc.Format, types.DefaultFormat(tc.Type))}
		if tc.Type == "datetime" {
			r.Timezone = tc.Timezone
		}
	}
	return r
}

// scope resolves the names a column's formula uses.
type scope struct {
	p *Plan
	c *column
}

func (s scope) Column(name string) (int, expr.Type, bool) {
	for _, n := range s.p.nodes {
		if n.name == name {
			return n.id, n.typ, true
		}
	}
	return 0, expr.Empty, false
}

func (s scope) Value() (expr.Type, bool) { return s.c.typ, s.c.hasSource }

// compileFormulas compiles every formula and condition, prepares the
// converters they need and orders the columns so that formulas come after
// what they read.
func (p *Plan) compileFormulas(issues *config.Issues) {
	m := p.Mapping
	t := m.Target.Inline
	rules := map[int]config.Rule{}
	cols := map[int]config.TargetColumn{}
	slots, _ := m.Slots()
	for _, c := range p.nodes {
		if c.helper {
			h := m.Helpers[c.name]
			rules[c.id], cols[c.id] = h.Rule, h.Column(c.name)
		} else {
			rules[c.id], _ = m.RuleOf(c.name, slots)
			cols[c.id] = t.Columns[c.id]
		}
	}

	for _, c := range p.nodes {
		if c.conv == nil {
			continue // invalid type, already reported
		}
		tc, rule := cols[c.id], rules[c.id]
		ok := true
		compile := func(path, src string, check func(expr.Type) error) *expr.Program {
			prog, err := expr.Compile(src, scope{p, c})
			if err == nil {
				err = check(prog.Type)
			}
			if err != nil {
				issues.Errorf(path, "%v", err)
				ok = false
				return nil
			}
			if prog.Volatile {
				issues.Warnf(path, "today() and now() give a different result on every run")
			}
			for _, id := range prog.Columns {
				if !slices.Contains(c.refs, id) {
					c.refs = append(c.refs, id)
				}
			}
			c.aggs = append(c.aggs, prog.Aggregates...)
			return prog
		}
		value := func(t expr.Type) error { return expr.Assignable(t, c.typ) }
		condition := func(t expr.Type) error {
			if t != expr.Boolean && t != expr.Empty {
				return fmt.Errorf("a condition is true or false, but this gives a %s value: compare it, like [column] > 0", t)
			}
			return nil
		}
		result := func(path string, then *config.Scalar, formula string) outcome {
			if formula != "" {
				return outcome{prog: compile(path+"_formula", formula, value)}
			}
			out, err := defaultValue(tc, string(*then))
			if err != nil {
				issues.Errorf(path, "%q is not a valid %s for this column: %v", *then, tc.Type, err)
			}
			return outcome{out: out, empty: out == ""}
		}

		if src := rule.DefaultFormula; strings.TrimSpace(src) != "" {
			path := c.path + ".default_formula"
			prog, err := expr.Compile(src, noValue{scope{p, c}})
			if err == nil {
				err = expr.Assignable(prog.Type, c.typ)
			}
			switch {
			case err != nil:
				issues.Errorf(path, "%v", err)
				ok = false
			case len(prog.Columns) > 0 || prog.UsesRow:
				issues.Errorf(path, "a default formula gives one value for the whole file: read columns inside aggregates only, like first([column]) or lowest([column])")
				ok = false
			default:
				c.defaultFormula = prog
				c.aggs = append(c.aggs, prog.Aggregates...)
				c.defaultAggs = len(prog.Aggregates)
			}
		}

		switch {
		case strings.TrimSpace(rule.Formula) != "":
			c.formula = compile(c.path+".formula", rule.Formula, value)
		case rule.Cases != nil:
			cs := &cases{}
			for i, k := range rule.Cases {
				path := fmt.Sprintf("%s.cases[%d]", c.path, i)
				if strings.TrimSpace(k.When) == "" || (k.Then != nil) == (k.ThenFormula != "") {
					ok = false // reported by config.Validate
					continue
				}
				cs.when = append(cs.when, compile(path+".when", k.When, condition))
				cs.then = append(cs.then, result(path+".then", k.Then, k.ThenFormula))
			}
			if rule.Else != nil || rule.ElseFormula != "" {
				e := result(c.path+".else", rule.Else, rule.ElseFormula)
				cs.otherwise = &e
			}
			c.cases = cs
		case rule.Sequence != nil:
			seq := &sequence{start: 1, step: 1}
			if s := rule.Sequence.Start; s != nil {
				seq.start = *s
			}
			if s := rule.Sequence.Step; s != nil {
				seq.step = *s
			}
			if !slices.Contains([]string{"integer", "decimal", "string"}, tc.Type) {
				issues.Errorf(c.path+".sequence", "row numbers fill integer, decimal or string columns, not %s", tc.Type)
				ok = false
			}
			c.seq = seq
		}
		if !ok || !c.computes() && c.defaultFormula == nil {
			continue // the plan fails, or there is nothing to compute
		}

		var errs []error
		if c.formula != nil && c.hasSource {
			conv, err := types.New(canonicalColumn(tc), &rule)
			c.parse, errs = conv, append(errs, err)
		}
		conv, err := types.New(tc, canonicalRule(tc))
		c.format, errs = conv, append(errs, err)
		if err := errors.Join(errs...); err != nil {
			issues.Errorf(c.path, "cannot prepare the formula: %v", err) // a bug: the column itself is valid
		}
	}

	p.compileAggregates(issues)

	// Helpers ordering a batch's rows are read like formulas read them.
	var orderBy []int
	if g := m.Group; g != nil {
		for _, c := range p.nodes {
			if c.helper && slices.Contains(g.OrderBy, c.name) {
				orderBy = append(orderBy, c.id)
			}
		}
	}

	for _, c := range append(p.nodes, nil) {
		reads := orderBy
		if c != nil {
			reads = c.reads()
		}
		for _, id := range reads {
			n := p.nodes[id]
			switch {
			case c == nil:
			case n == c && slices.Contains(c.refs, id):
				issues.Errorf(c.formulaPath(), "[%s] is this column: use value for its source value", c.name)
				continue
			case n.seq != nil:
				issues.Errorf(c.formulaPath(), "[%s] is numbered when rows are written: formulas cannot read it (row() is the input row number)", n.name)
				continue
			}
			if n.referenced || n.conv == nil {
				continue
			}
			n.referenced = true
			tc := cols[n.id]
			conv, err := types.New(canonicalColumn(tc), outputRule(tc))
			if err != nil {
				issues.Errorf(n.path, "cannot be read by formulas: %v", err) // a bug: the column itself is valid
			}
			n.back = conv
		}
	}

	for _, c := range p.nodes {
		if c.helper && !c.referenced {
			issues.Warnf(c.path, "no formula uses this helper")
		}
	}
	p.orderColumns(issues)
}

// cases are the conditions of a column: the first true one gives the
// value.
type cases struct {
	when      []*expr.Program
	then      []outcome
	otherwise *outcome // nil without else
}

// outcome is the value of a case: fixed (in output form) or a formula.
type outcome struct {
	out   string
	empty bool
	prog  *expr.Program
}

// sequence numbers the written rows.
type sequence struct{ start, step int64 }

// noValue is the scope of a default formula: no source value.
type noValue struct{ expr.Scope }

func (noValue) Value() (expr.Type, bool) { return expr.Empty, false }

// reads lists the columns the column's formulas read, aggregates included.
func (c *column) reads() []int {
	ids := slices.Clone(c.refs)
	for _, a := range c.aggs {
		for _, id := range a.Columns() {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// compileAggregates numbers the aggregates of every formula (their slot in
// expr.Env.Aggregates) and lists the columns the first pass computes: the
// ones their arguments read. An aggregate cannot read a column computed
// from an aggregate: one pass must be enough.
func (p *Plan) compileAggregates(issues *config.Issues) {
	// needs reports whether a column's value depends on an aggregate. The
	// aggregates of its default formula do not count: the first pass
	// computes columns without them, which changes only cells that would
	// get the default, and aggregates leave those out as empty. So a column
	// can default to the lowest of its own values.
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

	pass := map[int]bool{}
	var add func(id int)
	add = func(id int) {
		if pass[id] {
			return
		}
		pass[id] = true
		for _, r := range p.nodes[id].refs {
			add(r)
		}
	}
	for _, c := range p.nodes {
		for _, a := range c.aggs {
			a.Slot = len(p.aggregates)
			p.aggregates = append(p.aggregates, a)
			for _, id := range a.Columns() {
				n := p.nodes[id]
				if needs(n) {
					issues.Errorf(c.aggregatePath(), "%s([%s]): [%s] is itself computed from an aggregate, so it is not known while aggregates are computed", a.Kind, n.name, n.name)
					continue
				}
				add(id)
			}
		}
	}
	p.passCols = pass
}

// aggregatePath locates problems of a column's aggregates in issues.
func (c *column) aggregatePath() string {
	if c.formula == nil && c.cases == nil && c.defaultFormula != nil {
		return c.path + ".default_formula"
	}
	return c.formulaPath()
}

// formulaPath locates problems of the column's formulas in issues.
func (c *column) formulaPath() string {
	if c.formula != nil {
		return c.path + ".formula"
	}
	return c.path
}

// computes reports whether the column computes its value rather than
// reading it as is.
func (c *column) computes() bool { return c.formula != nil || c.cases != nil || c.seq != nil }

// orderColumns sorts columns so that each formula comes after the columns
// it reads, keeping the target order otherwise. Circular references are
// errors.
func (p *Plan) orderColumns(issues *config.Issues) {
	const (
		unvisited = iota
		visiting
		done
	)
	state := make([]int, len(p.nodes))
	var stack []string
	var visit func(c *column) bool
	visit = func(c *column) bool {
		switch state[c.id] {
		case done:
			return true
		case visiting:
			i := 0
			for stack[i] != c.name {
				i++
			}
			cycle := append(stack[i:], c.name)
			issues.Errorf(c.formulaPath(), "circular reference: %s", strings.Join(brackets(cycle), " → "))
			return false
		}
		state[c.id] = visiting
		stack = append(stack, c.name)
		for _, id := range c.refs {
			if id != c.id && !visit(p.nodes[id]) {
				return false
			}
		}
		stack = stack[:len(stack)-1]
		state[c.id] = done
		p.order = append(p.order, c.id)
		return true
	}
	for _, c := range p.nodes {
		if !visit(c) {
			return
		}
	}
}

func brackets(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "[" + n + "]"
	}
	return out
}

// toValue reads the value a column wrote, for the formulas reading it.
func (c *column) toValue(out string, empty bool) (expr.Value, error) {
	if empty {
		return expr.Value{}, nil
	}
	canon, err := c.back.Convert(out)
	if err != nil {
		return expr.Value{}, fmt.Errorf("formulas cannot read %q: %v", out, err)
	}
	return expr.Parse(c.typ, canon)
}
