package engine

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/expr"
	"github.com/S7eezy/lmmtfy/internal/types"
)

// Trace records how one column is filled, row by row: what Studio shows
// next to a formula.
type Trace struct {
	Column string // target column or helper
	Max    int    // rows to record
	Rows   []TraceRow
}

// TraceRow is one row of a trace.
type TraceRow struct {
	Line   int          `json:"line"`
	Value  string       `json:"value,omitempty"`  // source value, as read
	Inputs []TraceInput `json:"inputs,omitempty"` // columns the formula reads
	Result string       `json:"result"`
	Error  string       `json:"error,omitempty"`
}

// TraceInput is the value of a column a formula reads.
type TraceInput struct {
	Column string `json:"column"`
	Value  string `json:"value"`
	Empty  bool   `json:"empty,omitempty"`
}

func (r *run) traceRow(c *active, fields []string, w *worker, out string, err error) *TraceRow {
	row := &TraceRow{Result: out}
	if c.hasSource && !c.constant && !c.absent && c.idx < len(fields) {
		row.Value = fields[c.idx]
	}
	if c.refs != nil {
		for _, id := range c.refs {
			v := w.env.Columns[id]
			row.Inputs = append(row.Inputs, TraceInput{Column: r.plan.nodes[id].name, Value: v.Text(), Empty: v.IsEmpty()})
		}
	}
	var warn *types.Warning
	switch {
	case errors.As(err, &warn):
		row.Error = warn.Msg // written anyway
	case err != nil:
		row.Error, row.Result = err.Error(), ""
	}
	return row
}

// CheckFormula compiles the formula of one column (path "columns.<name>"
// or "helpers.<name>") on its own, for editors checking it as it is typed:
// the rest of the mapping may be incomplete. It returns the result type.
// With a path ending in ".when", the formula is one of the column's
// conditions, which must give true or false; with ".default_formula", its
// default formula, which reads columns only inside aggregates.
func CheckFormula(m *config.Mapping, path, src string) (string, error) {
	path, condition := strings.CutSuffix(path, ".when")
	path, isDefault := strings.CutSuffix(path, ".default_formula")
	p := &Plan{Mapping: m}
	var self *column
	add := func(name, typ, path string, rule config.Rule) {
		t, _ := expr.TypeOf(typ)
		c := &column{id: len(p.nodes), name: name, path: path, typ: t, hasSource: rule.HasSource(), slot: -1,
			helper: strings.HasPrefix(path, "helpers.")}
		p.nodes = append(p.nodes, c)
	}
	if t := m.Target.Inline; t != nil {
		for _, tc := range t.Columns {
			add(tc.Name, tc.Type, "columns."+tc.Name, m.Columns[tc.Name])
		}
	}
	for _, name := range slices.Sorted(maps.Keys(m.Helpers)) {
		h := m.Helpers[name]
		add(name, h.Type, "helpers."+name, h.Rule)
	}
	for _, c := range p.nodes {
		if c.path == path {
			self = c
		}
	}
	if self == nil {
		// A pattern (price_{n}) stands for its columns: typed like them.
		self = &column{path: path, slot: -1}
		if key, ok := strings.CutPrefix(path, "columns."); ok && config.IsPattern(key) {
			slots, _ := m.Slots()
			for name, s := range slots.Of {
				if s.Pattern == key {
					for _, c := range p.nodes {
						if c.name == name {
							self = &column{path: path, name: key, typ: c.typ, hasSource: m.Columns[key].HasSource(), slot: s.Index, id: -1}
						}
					}
				}
			}
		}
	}
	var sc expr.Scope = scope{p, self}
	if isDefault {
		sc = noValue{sc}
	}
	prog, err := expr.Compile(src, sc)
	if err != nil {
		return "", err
	}
	if isDefault && (len(prog.Columns) > 0 || prog.UsesRow) {
		return prog.Type.String(), &expr.Error{Msg: "a default formula gives one value for the whole file: read columns inside aggregates only, like first([column]) or lowest([column])"}
	}
	if slices.Contains(prog.Columns, self.id) && self.name != "" {
		return "", &expr.Error{Msg: "[" + self.name + "] is this column: use value for its source value"}
	}
	if m.Group != nil {
		if err := checkCombined(p, self, prog); err != nil {
			return prog.Type.String(), err
		}
	}
	if condition {
		if prog.Type != expr.Boolean && prog.Type != expr.Empty {
			return prog.Type.String(), &expr.Error{Msg: fmt.Sprintf("a condition is true or false, but this gives a %s value: compare it, like [column] > 0", prog.Type)}
		}
		return prog.Type.String(), nil
	}
	if err := expr.Assignable(prog.Type, self.typ); err != nil && self.name != "" {
		return prog.Type.String(), &expr.Error{Msg: err.Error()}
	}
	return prog.Type.String(), nil
}

// checkCombined applies the rules of combined rows to one formula, as
// compileGroup does for a whole mapping.
func checkCombined(p *Plan, self *column, prog *expr.Program) error {
	for _, id := range prog.Columns {
		n := p.nodes[id]
		switch {
		case self.helper && !n.helper:
			return &expr.Error{Msg: fmt.Sprintf("helpers are computed for each row of a batch, before output columns: [%s] is an output column", n.name)}
		case !self.helper && self.slot < 0 && n.helper:
			return &expr.Error{Msg: fmt.Sprintf("[%s] has one value per row of a batch: use first([%s]), last(…), lowest(…), total(…) or a pattern column", n.name, n.name)}
		}
	}
	for _, a := range prog.Aggregates {
		if self.helper {
			return &expr.Error{Msg: "helpers are computed for each row of a batch, before the batch's aggregates"}
		}
		for _, id := range a.Columns() {
			if n := p.nodes[id]; !n.helper {
				return &expr.Error{Msg: fmt.Sprintf("%s([%s]): aggregates of a batch read helpers, computed for each of its rows", a.Kind, n.name)}
			}
		}
	}
	return nil
}
