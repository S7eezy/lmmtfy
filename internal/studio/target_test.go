package studio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
)

func TestDraftTargetKeepsTheExampleFormat(t *testing.T) {
	dir := t.TempDir()
	example := filepath.Join(dir, "expected.csv")
	os.WriteFile(example, []byte("Order ID;Date;Amount;Paid\n1;05/10/2026;1 234,50;oui\n2;06/10/2026;99,90;non\n3;07/10/2026;10,00;oui\n"), 0o644)

	d, err := DraftTarget(example)
	if err != nil {
		t.Fatal(err)
	}
	tg := d.Target
	if o := tg.Output; o.Delimiter != ";" || o.LineEnding != "lf" || !*o.Header {
		t.Errorf("output = %+v", o)
	}
	var names []string
	for _, c := range tg.Columns {
		names = append(names, c.Name)
	}
	if want := []string{"Order ID", "Date", "Amount", "Paid"}; !slices.Equal(names, want) {
		t.Errorf("names = %v, want the headers as written %v", names, want)
	}
	if c := tg.Columns[1]; c.Type != "date" || c.Format != "DD/MM/YYYY" {
		t.Errorf("date column = %+v", c)
	}
	if c := tg.Columns[2]; c.Type != "decimal" || c.DecimalSep != "," || c.ThousandsSep != " " || c.Scale == nil || *c.Scale != 2 {
		t.Errorf("decimal column = %+v", c)
	}
	if issues := CheckTarget(tg); len(issues) > 0 {
		t.Errorf("drafted target has issues: %+v", issues)
	}
}

func TestCheckAndSaveTarget(t *testing.T) {
	dir := t.TempDir()
	tg, err := LoadTarget("../../testdata/acme-orders/target.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Output.Quote != `"` || !*tg.Output.Header {
		t.Errorf("defaults not applied: %+v", tg.Output)
	}
	if issues := CheckTarget(tg); len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}

	path := filepath.Join(dir, "erp.target.yaml")
	if _, err := SaveTarget(tg, path); err != nil {
		t.Fatal(err)
	}
	back, err := LoadTarget(path)
	if err != nil || len(back.Columns) != 6 || back.Columns[4].MaxLength == nil {
		t.Fatalf("saved target: %v %+v", err, back)
	}

	// Type options are checked, and invalid targets are not saved.
	tg.Columns[1].Format = "YYYY-QQ"
	tg.Columns = append(tg.Columns, config.TargetColumn{Name: "order_id", Type: "string"})
	issues, err := SaveTarget(tg, path)
	if err == nil {
		t.Fatal("invalid target saved")
	}
	text := ""
	for _, i := range issues {
		text += i.Path + ": " + i.Message + "\n"
	}
	if !strings.Contains(text, "target.columns.order_date") || !strings.Contains(text, "duplicate") {
		t.Errorf("issues:\n%s", text)
	}
}

func TestSavingAMappingLeavesItsLinkedTargetAlone(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "erp.target.yaml")
	original := []byte("# shared by several mappings\n" + mustRead(t, "../../testdata/acme-orders/target.yaml"))
	os.WriteFile(targetPath, original, 0o644)

	l, err := LoadMapping("../../testdata/acme-orders/mapping.yaml")
	if err != nil {
		t.Fatal(err)
	}
	mapping := filepath.Join(dir, "acme.mapping.yaml")
	if _, err := SaveMapping(fromEditor(t, l.Mapping), mapping, targetPath, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(targetPath); string(got) != string(original) {
		t.Errorf("the linked target file was rewritten:\n%s", got)
	}
	back, err := LoadMapping(mapping)
	if err != nil || back.TargetPath != targetPath {
		t.Fatalf("mapping does not refer to the target: %v %+v", err, back)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A rule without source and without a default of its own takes the target
// default; an explicit empty default overrides it. The editor relies on
// this: an empty default field means "the target default".
func TestTargetDefaultFillsColumnsWithoutSource(t *testing.T) {
	o, err := Open(acme)
	if err != nil {
		t.Fatal(err)
	}
	m := fromEditor(t, o.Mapping)
	def := config.Scalar("SENSOR_2")
	m.Target.Inline.Columns = append(m.Target.Inline.Columns, config.TargetColumn{Name: "sensor_name", Type: "string", Default: &def})

	value := func(rule *config.Rule) string {
		t.Helper()
		delete(m.Columns, "sensor_name")
		if rule != nil {
			m.Columns["sensor_name"] = *rule
		}
		res, err := PreviewOutput(acme, fromEditor(t, m))
		if err != nil || res.Failure != "" || len(res.Rows) == 0 {
			t.Fatalf("preview: %v %+v", err, res)
		}
		return res.Rows[0][len(res.Header)-1]
	}
	empty := config.Scalar("")
	if got := value(nil); got != "SENSOR_2" {
		t.Errorf("not mapped: %q, want the target default", got)
	}
	if got := value(&config.Rule{}); got != "SENSOR_2" {
		t.Errorf("rule without default: %q, want the target default", got)
	}
	if got := value(&config.Rule{Default: &empty}); got != "" {
		t.Errorf("empty default: %q, want empty", got)
	}
}
