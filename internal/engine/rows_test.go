package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
)

func compileRowsCase(t *testing.T, extra string) (*Plan, config.Issues) {
	t.Helper()
	m, err := config.ParseMapping([]byte(`
version: 1
name: r
target:
  output: { line_ending: lf }
  columns:
    - { name: id, type: integer }
    - { name: n, type: integer }
    - { name: amount, type: decimal, scale: 2 }
` + extra))
	if err != nil {
		t.Fatal(err)
	}
	m.ApplyDefaults()
	return Compile(m)
}

func TestKeepRowsValidation(t *testing.T) {
	for _, c := range []struct{ extra, want string }{
		{`keep_rows_when: "[id] + 1"
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, "keep_rows_when: a condition is true or false"},
		{`keep_rows_when: "[n] > 1"
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, "keep_rows_when: [n] is numbered when rows are written"},
		{`keep_rows_when: "[id] > mean([id])"
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, "keep_rows_when: aggregates cover the rows that are kept"},
		{`keep_rows_when: "[amount] > 1"
columns: { id: { source: id }, n: { sequence: {} }, amount: { formula: "[id] / count()" } }`, "keep_rows_when: [amount] is computed from an aggregate"},
		{`keep_rows_when: "[nope] > 1"
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, "keep_rows_when: "},
		{`duplicates: { key: [n] }
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, "duplicates.key: [n] is a row number"},
		{`duplicates: { key: [nope], keep: middle }
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, `duplicates.key[0]: "nope" is not a target column`},
		{`duplicates: { key: [] }
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`, "duplicates.key: list the target columns"},
	} {
		plan, issues := compileRowsCase(t, c.extra)
		if got := fmt.Sprint(issues); plan != nil || !strings.Contains(got, c.want) {
			t.Errorf("%s\nissues %v\nwant %q", c.extra, got, c.want)
		}
	}
}

func TestKeepRowsCombinedReadsHelpers(t *testing.T) {
	_, issues := compileGroupCase(t, `group: { by: [k] }
keep_rows_when: "[k] <> \"x\""
columns:
  k: { source: k }
  "v_{n}": { source: v }
  sum: { formula: "total([v])" }`)
	if got := fmt.Sprint(issues); !strings.Contains(got, "the condition is checked for each input row: it reads helpers") {
		t.Errorf("issues %v", got)
	}
}

// The condition's columns are computed first: a dropped row reports no
// error from other columns, and one whose condition reads an error is
// kept with that error.
func TestKeepRowsOrderAndErrors(t *testing.T) {
	plan, issues := compileRowsCase(t, `on_error: { strategy: skip_row }
keep_rows_when: "[id] <> 2"
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`)
	if plan == nil {
		t.Fatal(issues)
	}
	out, rej, err := runGroup(t, plan, "a,id\nx,1\nx,2\n1,3\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "id,n,amount\n3,1,1.00\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
	if !strings.Contains(rej, "x,1,") || strings.Contains(rej, "x,2,") {
		t.Errorf("rejects %q: want line 1 only", rej)
	}

	plan, _ = compileRowsCase(t, `on_error: { strategy: skip_row }
keep_rows_when: "[id] <> 2"
columns: { id: { source: id }, n: { sequence: {} }, amount: { source: a } }`)
	_, rej, _ = runGroup(t, plan, "a,id\n1,abc\n")
	if !strings.Contains(rej, "abc") {
		t.Errorf("a row whose condition cannot be decided is kept with its error; rejects %q", rej)
	}
}

func TestKeepRowsAggregates(t *testing.T) {
	plan, issues := compileRowsCase(t, `keep_rows_when: "[id] < 10"
columns: { id: { source: id }, n: { formula: "count()" }, amount: { formula: "mean([id])" } }`)
	if plan == nil {
		t.Fatal(issues)
	}
	out, _, err := runGroup(t, plan, "id\n1\n2\n30\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "id,n,amount\n1,2,1.50\n2,2,1.50\n"; out != want {
		t.Errorf("aggregates cover kept rows only: output %q, want %q", out, want)
	}
}

func TestSuggest(t *testing.T) {
	for _, c := range []struct {
		name  string
		among []string
		want  string
	}{
		{"Order date", []string{"id", "ORDER_DATE"}, "ORDER_DATE"},
		{"Prénom", []string{"prenom", "nom"}, "prenom"},
		{"amount", []string{"amout", "amounts"}, "amout"},
		{"amount", []string{"price", "qty"}, ""},
		{"id", []string{"ix", "name"}, "ix"},
		{"id", []string{"name"}, ""},
		{"", []string{"a"}, ""},
	} {
		if got := suggest(c.name, c.among); got != c.want {
			t.Errorf("suggest(%q, %q) = %q, want %q", c.name, c.among, got, c.want)
		}
	}
}
