package engine

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
)

func compileGroupCase(t *testing.T, extra string) (*Plan, config.Issues) {
	t.Helper()
	m, err := config.ParseMapping([]byte(`
version: 1
name: g
target:
  output: { line_ending: lf }
  columns:
    - { name: k, type: string }
    - { name: v_1, type: integer }
    - { name: v_2, type: integer }
    - { name: sum, type: integer }
helpers:
  v: { source: v, type: integer }
` + extra))
	if err != nil {
		t.Fatal(err)
	}
	m.ApplyDefaults()
	return Compile(m)
}

func runGroup(t *testing.T, plan *Plan, input string) (string, string, error) {
	t.Helper()
	var out, rej bytes.Buffer
	_, err := plan.Run(Job{Input: strings.NewReader(input), Output: &out, Rejects: func() (io.Writer, error) { return &rej, nil }})
	return out.String(), rej.String(), err
}

func TestGroupSortedAndOverflow(t *testing.T) {
	cols := `columns:
  k: { source: k }
  "v_{n}": { formula: "[v]" }
  sum: { formula: "total([v])" }
`
	plan, issues := compileGroupCase(t, "group: { by: [k] }\n"+cols)
	if plan == nil {
		t.Fatal(issues)
	}
	out, _, err := runGroup(t, plan, "k,v\na,1\na,2\nb,3\n")
	if err != nil || out != "k,v_1,v_2,sum\na,1,2,3\nb,3,,3\n" {
		t.Errorf("sorted: %q %v", out, err)
	}
	_, _, err = runGroup(t, plan, "k,v\na,1\nb,2\na,3\n")
	if err == nil || !strings.Contains(err.Error(), `line 4: the batch "a" starts again (first seen at line 2): the file is not sorted by k`) {
		t.Errorf("unsorted input: %v", err)
	}
	_, _, err = runGroup(t, plan, "k,v\na,1\na,2\na,3\n")
	if err == nil || !strings.Contains(err.Error(), `the batch "a" has 3 rows, more than the 2 slots (group.overflow: error)`) {
		t.Errorf("overflow: %v", err)
	}

	plan, issues = compileGroupCase(t, "group: { by: [k], sorted: false }\non_error: { strategy: skip_row }\n"+cols)
	if plan == nil {
		t.Fatal(issues)
	}
	out, rej, err := runGroup(t, plan, "k,v\na,1\nb,x\na,3\nb,4\n")
	if err != nil || out != "k,v_1,v_2,sum\na,1,3,4\n" {
		t.Errorf("unsorted: %q %v", out, err)
	}
	if rej != "k,v,_line,_column,_error\nb,x,3,v,\"\"\"x\"\" is not an integer\"\nb,4,5,v,\"\"\"x\"\" is not an integer\"\n" {
		t.Errorf("a rejected batch rejects all its rows: %q", rej)
	}
}

func TestGroupValidation(t *testing.T) {
	for _, c := range []struct{ extra, want string }{
		{`group: { by: [k] }
columns:
  k: { source: k }
  v_1: { source: v }
  v_2: { formula: "[v]" }
  sum: { formula: "[v] * 2" }`, ""},
		{`group: { by: [k] }
columns:
  k: { source: k }
  "v_{n}": { formula: "[v]" }
  sum: { formula: "[v] * 2" }`, "columns.sum.formula: [v] has one value per row of a batch: use first([v])"},
		{`group: { by: [k] }
columns:
  k: { source: k }
  "v_{n}": { source: v }
  sum: { formula: "count([k])" }`, "aggregates of a batch read helpers"},
		{`columns:
  k: { source: k }
  "v_{n}": { source: v }
  sum: { default: "0" }`, "patterns fill one column per row of a batch: they need group"},
		{`group: { by: [k], order_by: [nope] }
columns:
  k: { source: k }
  "v_{n}": { source: v }
  "x_{n}": { source: v }
  sum: { default: "0" }`, `"nope" is not a helper`},
	} {
		_, issues := compileGroupCase(t, c.extra)
		got := fmt.Sprint(issues)
		if c.want == "" {
			// v_1 reads a source that is not the key.
			c.want = "columns.v_1.source: a batch has several rows"
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s\nissues %v\nwant %q", c.extra, got, c.want)
		}
	}
}

// Combining rows by a key unique to each row changes nothing.
func TestGroupOfOneIsNoGroup(t *testing.T) {
	compile := func(src string) *Plan {
		m, err := config.ParseMapping([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		m.ApplyDefaults()
		p, issues := Compile(m)
		if p == nil {
			t.Fatal(issues)
		}
		return p
	}
	target := "version: 1\nname: m\ntarget:\n  output: { line_ending: lf }\n  columns: [{ name: id, type: integer }, { name: d, type: decimal, scale: 1 }, { name: n, type: integer }]\nhelpers:\n  dd: { source: d, type: decimal }\n"
	plain := compile(target + "columns:\n  id: { source: id }\n  d: { formula: \"[dd] * 2\" }\n  n: { sequence: {} }\n")
	grouped := compile(target + "group: { by: [id] }\ncolumns:\n  id: { source: id }\n  d: { formula: \"first([dd]) * 2\" }\n  n: { sequence: {} }\n")
	input := "id,d\n1,1.5\n2,\n3,-4\n"
	a, _, err1 := runGroup(t, plain, input)
	b, _, err2 := runGroup(t, grouped, input)
	if a != b || err1 != nil || err2 != nil {
		t.Errorf("plain %q %v\ngrouped %q %v", a, err1, b, err2)
	}
}

func TestCheckFormulaCombined(t *testing.T) {
	m, _ := config.ParseMapping([]byte(`
version: 1
name: g
group: { by: [k] }
target:
  columns: [{ name: k, type: string }, { name: v_1, type: integer }, { name: v_2, type: integer }, { name: sum, type: integer }]
helpers:
  v: { source: v, type: integer }
columns:
  k: { source: k }
  "v_{n}": { formula: "[v]" }
  sum: { formula: "total([v])" }
`))
	m.ApplyDefaults()
	for _, c := range []struct{ path, src, err string }{
		{"columns.sum", "total([v])", ""},
		{"columns.sum", "[v]", "has one value per row of a batch"},
		{"columns.sum", "lowest([v_1])", "aggregates of a batch read helpers"},
		{"columns.v_{n}", "[v] * 2", ""},
		{"helpers.v", "[k]", "helpers are computed for each row"},
	} {
		_, err := CheckFormula(m, c.path, c.src)
		if (err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s %q: %v, want %q", c.path, c.src, err, c.err)
		}
	}
}
