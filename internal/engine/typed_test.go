package engine

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/expr"
	"github.com/S7eezy/lmmtfy/internal/types"
)

// A formula reading a column gets its written value back (back), and a
// formula result is written through format: for any column, writing what
// was read gives the same output.
func TestTypedRoundTrip(t *testing.T) {
	s := func(v string) *config.Scalar { x := config.Scalar(v); return &x }
	scale := 2
	cases := []struct {
		col  config.TargetColumn
		outs []string
	}{
		{config.TargetColumn{Type: "string", Case: "upper"}, []string{"ABC", "ÜNÏCODE ✓"}},
		{config.TargetColumn{Type: "integer", ThousandsSep: " "}, []string{"1 234 567", "-12", "0"}},
		{config.TargetColumn{Type: "decimal", Scale: &scale, DecimalSep: ",", ThousandsSep: "."}, []string{"1.234,50", "-0,05", "0,00"}},
		{config.TargetColumn{Type: "decimal"}, []string{"0.000000054", "12", "-3.5"}},
		{config.TargetColumn{Type: "boolean", TrueValue: s("Y"), FalseValue: s("N")}, []string{"Y", "N"}},
		{config.TargetColumn{Type: "date", Format: "DD/MM/YYYY"}, []string{"31/01/2024", "29/02/2000"}},
		{config.TargetColumn{Type: "date"}, []string{"2024-01-31"}},
		{config.TargetColumn{Type: "datetime", Format: "YYYY-MM-DD HH:mm:ss.SSS", Timezone: "Europe/Paris"}, []string{"2024-03-31 03:30:00.123", "2024-07-01 23:59:59.999"}},
		{config.TargetColumn{Type: "datetime", Format: "YYYY-MM-DDTHH:mm:ssZ"}, []string{"2024-01-31T10:00:00Z"}},
		{config.TargetColumn{Type: "time", Format: "hh:mm A"}, []string{"02:05 PM", "12:00 AM"}},
	}
	for _, c := range cases {
		c.col.Name = "x"
		back, err := types.New(canonicalColumn(c.col), outputRule(c.col))
		if err != nil {
			t.Fatalf("%+v: back: %v", c.col, err)
		}
		format, err := types.New(c.col, canonicalRule(c.col))
		if err != nil {
			t.Fatalf("%+v: format: %v", c.col, err)
		}
		typ, _ := expr.TypeOf(c.col.Type)
		col := &column{typ: typ, back: back}
		for _, out := range c.outs {
			v, err := col.toValue(out, false)
			if err != nil {
				t.Errorf("%s %q: %v", c.col.Type, out, err)
				continue
			}
			got, err := format.Convert(v.Canonical())
			if err != nil || got != out {
				t.Errorf("%s %q: read %s, written back as %q (%v)", c.col.Type, out, v, got, err)
			}
		}
	}
}

// The canonical formats are valid lmmtfy formats and match what expr reads.
func TestCanonicalFormats(t *testing.T) {
	for _, f := range []string{expr.DateFormat, expr.DatetimeFormat, expr.TimeFormat} {
		if _, err := types.Layout(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func formulaMapping(t *testing.T) *config.Mapping {
	t.Helper()
	m, err := config.ParseMapping([]byte(`
version: 1
name: m
target:
  columns:
    - { name: id, type: integer }
    - { name: unit, type: decimal, scale: 2 }
    - { name: label, type: string }
helpers:
  total: { source: total, type: decimal }
  qty: { source: qty, type: integer }
columns:
  id: { source: id }
  unit: { formula: "[total] / [qty]" }
  label: { source: label, formula: 'upper(value) & "-" & [id]' }
`))
	if err != nil {
		t.Fatal(err)
	}
	m.ApplyDefaults()
	return m
}

func TestTrace(t *testing.T) {
	plan, issues := Compile(formulaMapping(t))
	if plan == nil {
		t.Fatal(issues)
	}
	input := "id,total,qty,label\n1,10,4,ab\n2,1,0,cd\n3,5,2,\n"
	trace := &Trace{Column: "unit", Max: 10}
	var out bytes.Buffer
	if _, err := plan.Run(Job{Input: strings.NewReader(input), Output: &out, Trace: trace, Workers: 2}); err == nil {
		t.Fatal("expected the division by zero to fail the file")
	}
	if len(trace.Rows) != 2 {
		t.Fatalf("rows: %+v", trace.Rows)
	}
	if r := trace.Rows[0]; r.Line != 2 || r.Result != "2.50" || len(r.Inputs) != 2 || r.Inputs[0].Value != "10" || r.Inputs[1].Value != "4" {
		t.Errorf("row 1: %+v", r)
	}
	if r := trace.Rows[1]; r.Error != "division by zero" || r.Result != "" {
		t.Errorf("row 2: %+v", r)
	}

	trace = &Trace{Column: "label", Max: 1}
	plan.Run(Job{Input: strings.NewReader(input), Output: &out, Trace: trace})
	if r := trace.Rows; len(r) != 1 || r[0].Value != "ab" || r[0].Result != "AB-1" {
		t.Errorf("label: %+v", r)
	}
}

func TestCheckFormula(t *testing.T) {
	m := formulaMapping(t)
	for _, c := range []struct{ path, src, typ, err string }{
		{"columns.unit", "[total] / [qty]", "decimal", ""},
		{"columns.label", "value & [id]", "string", ""},
		{"columns.unit", "value * 2", "", "has no source"},
		{"columns.id", "[total]", "decimal", "use round"},
		{"columns.unit", "[unit] + 1", "", "is this column"},
		{"helpers.qty", "value * 2", "integer", ""},
		{"columns.label", "[nope]", "", "unknown column [nope]"},
		{"columns.id.when", "[total] > 1", "boolean", ""},
		{"columns.id.when", "[total]", "decimal", "a condition is true or false"},
	} {
		typ, err := CheckFormula(m, c.path, c.src)
		if typ != c.typ || (err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s %q: %q, %v; want %q, %q", c.path, c.src, typ, err, c.typ, c.err)
		}
	}
}

func TestCasesAndSequenceErrors(t *testing.T) {
	for _, c := range []struct{ columns, want string }{
		{`n: { sequence: {} }
  s: { formula: "text([n])" }`, "columns.s.formula: [n] is numbered when rows are written"},
		{`n: { cases: [{ when: "[s]", then: "1" }] }
  s: { source: a }`, "columns.n.cases[0].when: a condition is true or false, but this gives a string value"},
		{`n: { cases: [{ when: "true", then: "x" }] }
  s: { source: a }`, `columns.n.cases[0].then: "x" is not a valid integer`},
		{`n: { sequence: {}, source: a }
  s: { source: a }`, "columns.n: a column is filled one way"},
		{`n: { source: a, else: "1" }
  s: { source: a }`, "columns.n.else: has no effect without cases"},
		{`n: { sequence: { step: 0 } }
  s: { source: a }`, "columns.n.sequence.step: must not be 0"},
	} {
		m, err := config.ParseMapping([]byte("version: 1\nname: m\ntarget:\n  columns:\n    - { name: n, type: integer }\n    - { name: s, type: string }\ncolumns:\n  " + c.columns + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		m.ApplyDefaults()
		_, issues := Compile(m)
		if !strings.Contains(fmt.Sprint(issues), c.want) {
			t.Errorf("%s\nissues %v\nwant %q", c.columns, issues, c.want)
		}
	}
}

func TestRequiredCaseWithoutMatch(t *testing.T) {
	m, _ := config.ParseMapping([]byte(`
version: 1
name: m
target:
  columns: [{ name: grade, type: string, required: true }]
columns:
  grade: { cases: [{ when: "row() > 1", then: B }] }
`))
	m.ApplyDefaults()
	plan, issues := Compile(m)
	if plan == nil {
		t.Fatal(issues)
	}
	var out bytes.Buffer
	_, err := plan.Run(Job{Input: strings.NewReader("x\n1\n2\n"), Output: &out})
	if err == nil || !strings.Contains(err.Error(), "no case matches, and there is no else") {
		t.Errorf("error %v, output %q", err, out.String())
	}
}

// Aggregates read the input twice: a pipe is copied first. With MaxRows
// (previews), aggregates cover the same rows, unless AggregateAll.
func TestAggregatesInputs(t *testing.T) {
	m, _ := config.ParseMapping([]byte(`
version: 1
name: m
target:
  output: { line_ending: lf }
  columns: [{ name: n, type: integer }, { name: top, type: integer }]
columns:
  n: { source: n }
  top: { formula: "highest([n])" }
`))
	m.ApplyDefaults()
	plan, issues := Compile(m)
	if plan == nil {
		t.Fatal(issues)
	}
	input := "n\n1\n5\n3\n"
	for _, c := range []struct {
		job  Job
		want string
	}{
		{Job{Input: io.MultiReader(strings.NewReader(input))}, "n,top\n1,5\n5,5\n3,5\n"},
		{Job{Input: strings.NewReader(input), MaxRows: 1}, "n,top\n1,1\n"},
		{Job{Input: strings.NewReader(input), MaxRows: 1, AggregateAll: true}, "n,top\n1,5\n"},
	} {
		var out bytes.Buffer
		c.job.Output = &out
		if _, err := plan.Run(c.job); err != nil {
			t.Fatal(err)
		}
		if out.String() != c.want {
			t.Errorf("got %q, want %q", out.String(), c.want)
		}
	}
}

func TestAggregateErrors(t *testing.T) {
	for _, c := range []struct{ columns, want string }{
		{`a: { formula: "total([b])" }
  b: { formula: "[c] * 2" }
  c: { formula: "lowest([a])" }`, "[b] is itself computed from an aggregate"},
		{`a: { source: x, default_formula: "[b]" }
  b: { source: y }
  c: { source: z }`, "a default formula gives one value for the whole file"},
		{`a: { source: x, default: "1", default_formula: "first([b])" }
  b: { source: y }
  c: { source: z }`, "default and default_formula are mutually exclusive"},
	} {
		m, err := config.ParseMapping([]byte("version: 1\nname: m\ntarget:\n  columns:\n    - { name: a, type: integer }\n    - { name: b, type: integer }\n    - { name: c, type: integer }\ncolumns:\n  " + c.columns + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		m.ApplyDefaults()
		_, issues := Compile(m)
		if !strings.Contains(fmt.Sprint(issues), c.want) {
			t.Errorf("%s\nissues %v\nwant %q", c.columns, issues, c.want)
		}
	}
}
