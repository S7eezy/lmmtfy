package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const target = `
version: 1
name: erp
columns:
  - { name: id, type: integer, required: true }
  - { name: code, type: string }
  - { name: country, type: string, default: FR }
`

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func load(t *testing.T, mapping string) *Mapping {
	t.Helper()
	dir := writeFiles(t, map[string]string{"target.yaml": target, "mapping.yaml": mapping})
	m, err := LoadMapping(filepath.Join(dir, "mapping.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestScalarKeepsSourceText(t *testing.T) {
	m := load(t, `
version: 1
name: m
target: target.yaml
columns:
  id:   { source: "01" }
  code:
    source: 2024
    default: 1.50
    map: { 01: one, yes: oui, "x y": z }
`)
	code := m.Columns["code"]
	if got := code.Source.String(); got != "2024" {
		t.Errorf("source = %q", got)
	}
	if got := code.Default.String(); got != "1.50" {
		t.Errorf("default = %q, want 1.50", got)
	}
	want := ValueMap{{"01", "one"}, {"yes", "oui"}, {"x y", "z"}}
	if len(code.Map) != len(want) {
		t.Fatalf("map = %v", code.Map)
	}
	for i := range want {
		if code.Map[i] != want[i] {
			t.Errorf("map[%d] = %v, want %v", i, code.Map[i], want[i])
		}
	}
}

func TestDefaultsApplied(t *testing.T) {
	m := load(t, `
version: 1
name: m
target: target.yaml
columns:
  id: { source: ID }
  code: { source: Code, optional: true }
`)
	if m.OnError.Strategy != "fail_file" || !*m.OnError.RejectFile {
		t.Errorf("on_error = %+v", m.OnError)
	}
	if m.Input.HeaderRow != 1 || m.Input.Delimiter != "," || !*m.Input.Header {
		t.Errorf("input = %+v", m.Input)
	}
	if got := m.Columns["code"].IfMissing; got != "empty" {
		t.Errorf("optional if_missing = %q, want empty", got)
	}
	if got := m.Columns["id"].IfMissing; got != "error" {
		t.Errorf("if_missing = %q, want error", got)
	}
	if got := m.Target.Inline.Output.LineEnding; got != "crlf" {
		t.Errorf("line_ending = %q", got)
	}
}

func TestUnknownFieldRejectedWithPosition(t *testing.T) {
	dir := writeFiles(t, map[string]string{"target.yaml": target, "mapping.yaml": `
version: 1
name: m
target: target.yaml
on_eror: { strategy: skip_row }
`})
	_, err := LoadMapping(filepath.Join(dir, "mapping.yaml"))
	if err == nil || !strings.Contains(err.Error(), `unknown field "on_eror"`) || !strings.Contains(err.Error(), "[5:1]") {
		t.Errorf("err = %v", err)
	}
}

func TestInlineTarget(t *testing.T) {
	dir := writeFiles(t, map[string]string{"mapping.yaml": `
version: 1
name: m
target:
  output: { delimiter: tab }
  columns:
    - { name: id, type: integer }
columns:
  id: { source: ID }
`})
	m, err := LoadMapping(filepath.Join(dir, "mapping.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := Char(m.Target.Inline.Output.Delimiter); got != "\t" {
		t.Errorf("delimiter = %q", got)
	}
	if is := Validate(m); is.HasErrors() {
		t.Errorf("unexpected issues: %v", is)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		columns string
		extra   string
		want    []string // substrings, one per expected issue
	}{
		{
			name:    "valid",
			columns: "id: { source: ID }",
			want:    []string{"warning: columns.code: not mapped: column left empty", `warning: columns.country: not mapped: every row gets the target default "FR"`},
		},
		{
			name:    "unknown target column",
			columns: "id: { source: ID }\n  nope: { source: X }\n  code: { source: C }\n  country: { source: K }",
			want:    []string{"error: columns.nope: no target column"},
		},
		{
			name:    "required not mapped",
			columns: "code: { source: C }\n  country: { source: K }",
			want:    []string{"error: columns.id: required target column is not mapped"},
		},
		{
			name:    "no source needs default",
			columns: "id: { source: ID }\n  code: {}\n  country: { source: K }",
			want:    []string{"error: columns.code.default: a column without source or formula requires a default"},
		},
		{
			name:    "omit warns",
			columns: "id: { source: ID }\n  code: { source: C, if_missing: omit }\n  country: { source: K }",
			want:    []string{"warning: columns.code.if_missing: omit removes the column"},
		},
		{
			name:    "bad enum and delimiter",
			columns: "id: { source: ID, if_empty: maybe }\n  code: { source: C }\n  country: { source: K }",
			extra:   "input: { delimiter: ';;' }\n",
			want:    []string{`error: input.delimiter: must be a single character`, `error: columns.id.if_empty: "maybe" is not one of`},
		},
		{
			name:    "source without header",
			columns: "id: { source: ID }\n  code: { source_index: 2 }\n  country: { source_index: 3 }",
			extra:   "input: { header: false }\n",
			want:    []string{"error: columns.id.source: input has no header"},
		},
		{
			name:    "formula needs no default",
			columns: "id: { source: ID }\n  code: { formula: '[n] & \"x\"' }\n  country: { source: K }",
			extra:   "helpers:\n  n: { source: N, type: integer }\n",
		},
		{
			name:    "helper problems",
			columns: "id: { source: ID }\n  code: { source: C }\n  country: { source: K }",
			extra:   "helpers:\n  code: { source: X, type: string }\n  t: { source: T }\n  o: { source: O, type: date, if_missing: omit }\n  d: { type: integer }\n",
			want: []string{
				"error: helpers.code: a target column has the same name",
				"error: helpers.t.type: required",
				"error: helpers.o.if_missing: omit only applies to output columns",
				"error: helpers.d.default: a column without source or formula requires a default",
			},
		},
		{
			name:    "required default strategy needs default",
			columns: "id: { source: ID, on_error: default }\n  code: { source: C }\n  country: { source: K }",
			want:    []string{"error: columns.id.on_error: on_error: default on a required column requires a default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := load(t, "version: 1\nname: m\ntarget: target.yaml\n"+tt.extra+"columns:\n  "+tt.columns+"\n")
			issues := Validate(m)
			if len(issues) != len(tt.want) {
				t.Fatalf("got %d issues, want %d:\n%v", len(issues), len(tt.want), issues)
			}
			for _, want := range tt.want {
				found := false
				for _, is := range issues {
					if strings.Contains(is.String(), want) {
						found = true
					}
				}
				if !found {
					t.Errorf("no issue containing %q in:\n%v", want, issues)
				}
			}
		})
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	m := load(t, `
version: 1
name: m
target: target.yaml
columns:
  id: { source: "01" }
  code: { source: C, map: { 01: one, B: two } }
`)
	data, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseMapping(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	back.ApplyDefaults()
	if got := back.Columns["code"].Map[0]; got != (Pair{"01", "one"}) {
		t.Errorf("map[0] = %v\n%s", got, data)
	}
	if got := back.Columns["id"].Source.String(); got != "01" {
		t.Errorf("source = %q\n%s", got, data)
	}
	h1, _ := Hash(m)
	h2, _ := Hash(back)
	if h1 != h2 {
		t.Errorf("hash changed after round trip\n%s", data)
	}
}

func TestHelpersRoundTrip(t *testing.T) {
	m := load(t, `
version: 1
name: m
target: target.yaml
helpers:
  temp: { source: "Temp", type: decimal, decimal_sep: ",", optional: true }
columns:
  id: { source: ID }
  code: { formula: 'if([temp] > 5, "HOT", "COLD")' }
  country: { source: K }
`)
	if h := m.Helpers["temp"]; h.Type != "decimal" || h.DecimalSep != "," || h.IfMissing != "empty" || h.Source.String() != "Temp" {
		t.Fatalf("helper = %+v", h)
	}
	if issues := Validate(m); len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	for _, format := range []func() ([]byte, error){
		func() ([]byte, error) { return Marshal(m) },
		func() ([]byte, error) { return Format(m, "target.yaml") },
	} {
		data, err := format()
		if err != nil {
			t.Fatal(err)
		}
		back, err := ParseMapping(data)
		if err != nil {
			t.Fatalf("%v\n%s", err, data)
		}
		if h := back.Helpers["temp"]; h.Type != "decimal" || h.DecimalSep != "," {
			t.Errorf("helper after round trip = %+v\n%s", h, data)
		}
		if f := back.Columns["code"].Formula; f != `if([temp] > 5, "HOT", "COLD")` {
			t.Errorf("formula after round trip = %q", f)
		}
	}
	if _, err := ParseMapping([]byte("version: 1\nname: m\ntarget: t.yaml\nhelpers:\n  x: { type: integer, typo: 1 }\ncolumns: {}\n")); err == nil {
		t.Error("unknown helper field accepted")
	}
}

func TestMalformedYAMLIsAnErrorNotAPanic(t *testing.T) {
	if _, err := ParseMapping([]byte("target:\n columns: ! ")); err == nil {
		t.Error("want an error")
	}
}

func TestGroupRoundTrip(t *testing.T) {
	m, err := LoadMapping("../../testdata/combine-orders/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if issues := Validate(m); len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	slots, _ := m.Slots()
	if len(slots.Numbers) != 4 || slots.Of["price_3"] != (Slot{Pattern: "price_{n}", Index: 3}) {
		t.Fatalf("slots: %+v", slots)
	}
	data, err := Format(m, "target.yaml")
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseMapping(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	if back.Group == nil || back.Group.By[0] != "batchId" || back.Group.Overflow != "drop_extra" {
		t.Errorf("group: %+v\n%s", back.Group, data)
	}
	// Patterns are written where their first column is, in target order.
	if i, j := strings.Index(string(data), "batchId:\n"), strings.Index(string(data), "object_{n}"); i < 0 || j < i {
		t.Errorf("order:\n%s", data)
	}
	if back.Columns["price_{n}"].Formula != "[price]" {
		t.Errorf("pattern rule lost:\n%s", data)
	}
}
