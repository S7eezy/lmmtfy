package studio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
)

const acme = "../../testdata/acme-orders/input.csv"

// fromEditor sends a mapping through JSON, as the editor does.
func fromEditor(t *testing.T, m *config.Mapping) *config.Mapping {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out config.Mapping
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func TestOpenProposesValidMapping(t *testing.T) {
	o, err := Open(acme)
	if err != nil {
		t.Fatal(err)
	}
	if o.Detection.Delimiter != ";" || o.Mapping.Input.HeaderRow != 3 {
		t.Errorf("detection: %+v", o.Detection)
	}
	if issues := Check(fromEditor(t, o.Mapping)); slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == "error" }) {
		t.Errorf("proposed mapping has errors: %+v", issues)
	}
}

func TestReadRawKinds(t *testing.T) {
	o, _ := Open(acme)
	raw, err := ReadRaw(acme, o.Mapping.Input)
	if err != nil || raw.Error != "" {
		t.Fatal(err, raw.Error)
	}
	var kinds []string
	for _, r := range raw.Rows {
		kinds = append(kinds, r.Kind)
	}
	want := []string{"skipped", "skipped", "header", "data", "data", "data", "footer"}
	if !slices.Equal(kinds, want) || raw.Width != 5 {
		t.Errorf("kinds = %v (width %d), want %v", kinds, raw.Width, want)
	}

	// The user changes the delimiter: the preview follows.
	in := o.Mapping.Input
	in.Delimiter = ","
	raw, _ = ReadRaw(acme, in)
	if raw.Width == 5 {
		t.Error("wrong delimiter should not give 5 columns")
	}
}

func TestPreviewOutputAndEditing(t *testing.T) {
	o, _ := Open(acme)
	m := fromEditor(t, o.Mapping)

	res, err := PreviewOutput(acme, m, PreviewOptions{})
	if err != nil || res.Failure != "" {
		t.Fatal(err, res.Failure)
	}
	if !slices.Equal(res.Header, []string{"N_Commande", "Date", "Montant_TTC", "Etat", "Remarque"}) || len(res.Rows) != 3 {
		t.Fatalf("header %v, %d rows", res.Header, len(res.Rows))
	}
	if got := res.Rows[0][2]; got != "1234.50" {
		t.Errorf("amount = %q", got)
	}

	// Rename a column, type it, delete one, add one with a default.
	cols := m.Target.Inline.Columns
	cols[0].Name = "order_id"
	m.Columns["order_id"] = m.Columns["N_Commande"]
	delete(m.Columns, "N_Commande")
	m.Target.Inline.Columns = append(slices.Delete(cols, 3, 4), config.TargetColumn{Name: "country", Type: "string"})
	delete(m.Columns, "Etat")
	fr := config.Scalar("FR")
	m.Columns["country"] = config.Rule{Default: &fr}

	res, _ = PreviewOutput(acme, m, PreviewOptions{})
	if res.Failure != "" || !slices.Equal(res.Header, []string{"order_id", "Date", "Montant_TTC", "Remarque", "country"}) || res.Rows[2][4] != "FR" {
		t.Errorf("edited preview: %v %v %q", res.Header, res.Rows, res.Failure)
	}

	// A broken rule shows issues, not a preview.
	bad := m.Columns["Date"]
	bad.Formats = []string{"DD/MM/2026"}
	m.Columns["Date"] = bad
	res, _ = PreviewOutput(acme, m, PreviewOptions{})
	if res.Rows != nil || !slices.ContainsFunc(res.Issues, func(i Issue) bool { return i.Path == "columns.Date.formats" }) {
		t.Errorf("issues = %+v", res.Issues)
	}
}

func TestPreviewShowsRejects(t *testing.T) {
	l, err := LoadMapping("../../testdata/skip-row-rejects/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	res, err := PreviewOutput("../../testdata/skip-row-rejects/input.csv", fromEditor(t, l.Mapping), PreviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 || len(res.Rejected) != 4 || res.Rejected[0][len(res.Rejected[0])-1] != "_error" {
		t.Errorf("rows %d, rejected %v", len(res.Rows), res.Rejected)
	}
	if len(res.Events) != 5 || res.Events[0].Column != "order_date" || res.Events[0].Line != 3 {
		t.Errorf("events = %+v", res.Events)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	l, err := LoadMapping("../../testdata/acme-orders/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(l.TargetPath, "acme-orders/target.yaml") {
		t.Errorf("target path = %q", l.TargetPath)
	}

	// Inline target.
	inline := filepath.Join(dir, "inline.yaml")
	if _, err := SaveMapping(fromEditor(t, l.Mapping), inline, "", false); err != nil {
		t.Fatal(err)
	}
	back, err := LoadMapping(inline)
	if err != nil || back.TargetPath != "" || len(back.Mapping.Target.Inline.Columns) != 6 {
		t.Fatalf("inline: %v %+v", err, back)
	}

	// Target in its own file, referenced relatively.
	mapping := filepath.Join(dir, "m", "acme.yaml")
	os.Mkdir(filepath.Dir(mapping), 0o755)
	targetPath := filepath.Join(dir, "erp.target.yaml")
	if _, err := SaveMapping(fromEditor(t, l.Mapping), mapping, targetPath, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(mapping)
	if !strings.Contains(string(data), "target: ../erp.target.yaml") {
		t.Errorf("mapping does not refer to its target:\n%s", data)
	}
	back, err = LoadMapping(mapping)
	if err != nil || back.TargetPath != targetPath {
		t.Fatalf("referenced: %v %+v", err, back)
	}
	h1, _ := config.Hash(l.Mapping)
	h2, _ := config.Hash(back.Mapping)
	if h1 != h2 {
		t.Error("saving and loading changed the mapping")
	}
}

func TestConvertAndExport(t *testing.T) {
	dir := t.TempDir()
	l, _ := LoadMapping("../../testdata/skip-row-rejects/mapping.yaml")
	out := filepath.Join(dir, "out.csv")
	res, err := Convert(fromEditor(t, l.Mapping), "../../testdata/skip-row-rejects/input.csv", out)
	if err != nil || res.Failure != "" {
		t.Fatal(err, res.Failure)
	}
	want, _ := os.ReadFile("../../testdata/skip-row-rejects/output.csv")
	got, _ := os.ReadFile(out)
	if string(got) != string(want) || res.Rejects != filepath.Join(dir, "out.rejects.csv") {
		t.Errorf("convert: rejects %q, output:\n%s", res.Rejects, got)
	}

	available := slices.ContainsFunc(Platforms(), func(p Platform) bool { return p.Available })
	if !available {
		t.Skip("no converter templates embedded: run `make build` first")
	}
	name := ConverterName(l.Mapping, "windows", "amd64")
	path, err := Export(fromEditor(t, l.Mapping), "windows", "amd64", filepath.Join(dir, strings.TrimSuffix(name, ".exe")))
	if err != nil || filepath.Base(path) != "skip-rows-converter.exe" {
		t.Errorf("export: %v, %s", err, path)
	}
}

func TestRecentFiles(t *testing.T) {
	dir := t.TempDir()
	saved := configDir
	defer func() { configDir = saved }()
	configDir = func() (string, error) { return filepath.Join(dir, "config"), nil }

	if got := RecentFiles(); len(got) != 0 {
		t.Fatalf("fresh: %v", got)
	}
	files := make([]string, maxRecent+2)
	for i := range files {
		files[i] = filepath.Join(dir, fmt.Sprintf("f%d.csv", i))
		os.WriteFile(files[i], nil, 0o644)
		if err := Remember("csv", files[i]); err != nil {
			t.Fatal(err)
		}
	}
	Remember("mapping", files[3]) // reopened: moves to the top, no duplicate
	os.Remove(files[maxRecent+1]) // deleted files are not listed

	got := RecentFiles()
	if len(got) != maxRecent-1 || got[0].Path != files[3] || got[0].Kind != "mapping" {
		t.Errorf("recent = %+v", got)
	}
}

func TestSaveUnfinishedMapping(t *testing.T) {
	l, err := LoadMapping("../../testdata/acme-orders/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := fromEditor(t, l.Mapping)
	broken := m.Columns["order_date"]
	broken.Formula = "[nowhere] +"
	m.Columns["order_date"] = broken

	path := filepath.Join(t.TempDir(), "draft.mapping.yaml")
	issues, err := SaveMapping(m, path, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == "error" && i.Path == "columns.order_date.formula" }) {
		t.Errorf("issues = %+v", issues)
	}
	back, err := LoadMapping(path)
	if err != nil || back.Mapping.Columns["order_date"].Formula != "[nowhere] +" {
		t.Fatalf("unfinished mapping does not load back: %v", err)
	}
	if _, issues := compile(back.Mapping); !slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == "error" }) {
		t.Error("errors gone after loading")
	}
}

// A previewed row can become a test: the preview gives the input rows of
// each output and rejected row, and a test made from them passes.
func TestPinRowsAsTests(t *testing.T) {
	const dir = "../../testdata/keep-rows-and-duplicates/"
	l, err := LoadMapping(dir + "mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := fromEditor(t, l.Mapping)
	res, err := PreviewOutput(dir+"input.csv", m, PreviewOptions{})
	if err != nil || res.Failure != "" {
		t.Fatal(err, res.Failure)
	}
	if len(res.Sources) != len(res.Rows) || len(res.RejectedSources) != len(res.Rejected)-1 {
		t.Fatalf("sources %d for %d rows, %d for %d rejected", len(res.Sources), len(res.Rows), len(res.RejectedSources), len(res.Rejected)-1)
	}
	if got := fmt.Sprint(res.Sources[1]); got != "[[{id 3} {status OK} {amount 12}]]" {
		t.Errorf("sources of row 2: %s", got)
	}

	for i, input := range [][]config.Row{res.Sources[1], res.RejectedSources[0]} {
		test, err := MakeTest(m, fmt.Sprint("pinned ", i), input)
		if err != nil {
			t.Fatal(err)
		}
		m.Tests = append(m.Tests, *test)
	}
	if e := m.Tests[len(m.Tests)-1].Errors; len(e) != 1 || !strings.HasPrefix(e[0], "amount: ") {
		t.Errorf("a rejected row's test expects its error: %q", e)
	}
	results := RunTests(m)
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	// The mapping's own tests: 3 of 4 pass. The pinned ones pass.
	if len(results) != 6 || passed != 5 {
		t.Errorf("results: %+v", results)
	}
}

func TestCheckInputSuggests(t *testing.T) {
	l, err := LoadMapping("../../testdata/missing-column-suggestion/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := CheckInput("../../testdata/missing-column-suggestion/input.csv", fromEditor(t, l.Mapping))
	if err != nil {
		t.Fatal(err)
	}
	if c.OK || c.Columns[1].Suggestion != "ORDER_DATE" {
		t.Errorf("check: %+v", c)
	}
}
