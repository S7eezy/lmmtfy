package studio

import (
	"encoding/json"
	"strings"
	"testing"
)

const formulas = "../../testdata/formulas/"

// A mapping with helpers and formulas survives the trip through the editor
// (JSON), previews like the CLI converts, and traces a column.
func TestFormulasInStudio(t *testing.T) {
	l, err := LoadMapping(formulas + "mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(l.Mapping)
	if !strings.Contains(string(data), `"helpers":{"first":{"type":"string","source":"first"`) {
		t.Errorf("helpers are not flat in JSON: %s", data)
	}
	m := fromEditor(t, l.Mapping)

	res, err := PreviewOutput(formulas+"input.csv", m, PreviewOptions{Trace: "total"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failure != "" || len(res.Rows) != 3 {
		t.Fatalf("failure %q, rows %v, issues %v", res.Failure, res.Rows, res.Issues)
	}
	if got := strings.Join(res.Rows[1], ","); got != "2,2,2023-11-14 22:14:20.000,1700000060.000,COLD,2.50,2.50,ALAN TURING,00002-xx,XX" {
		t.Errorf("row 2: %s", got)
	}
	if len(res.Trace) != 3 {
		t.Fatalf("trace: %+v", res.Trace)
	}
	first := res.Trace[0]
	if first.Result != "30.03" || len(first.Inputs) != 2 || first.Inputs[0].Column != "price" || first.Inputs[0].Value != "10.01" {
		t.Errorf("trace row 1: %+v", first)
	}
	if in := res.Trace[1].Inputs[1]; in.Column != "qty" || !in.Empty {
		t.Errorf("empty qty: %+v", in)
	}

	if c := CheckFormula(m, "columns.total", "[price] * [qty]"); c.Type != "decimal" || c.Error != "" {
		t.Errorf("check: %+v", c)
	}
	if c := CheckFormula(m, "columns.total", "[price] *"); c.Error != "unexpected end of formula" || c.Pos != 10 {
		t.Errorf("check error: %+v", c)
	}
	if len(Functions()) < 30 {
		t.Error("functions are missing")
	}
}

func TestBatches(t *testing.T) {
	l, err := LoadMapping("../../testdata/combine-orders/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	st, err := Batches("../../testdata/combine-orders/input.csv", l.Mapping, []string{"batchId"})
	if err != nil {
		t.Fatal(err)
	}
	if *st != (BatchStats{Rows: 10, Batches: 4, Min: 1, Max: 5, Sorted: true}) {
		t.Errorf("%+v", st)
	}
	if st, _ := Batches("../../testdata/combine-orders/input.csv", l.Mapping, []string{"object"}); st.Batches != 10 || st.Max != 1 {
		t.Errorf("%+v", st)
	}
	if _, err := Batches("../../testdata/combine-orders/input.csv", l.Mapping, []string{"nope"}); err == nil {
		t.Error("unknown column accepted")
	}
}
