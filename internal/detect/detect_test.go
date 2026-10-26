package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lmmtfy/internal/config"
	"lmmtfy/internal/engine"
	"lmmtfy/internal/golden"
)

// Every golden case has a hand-written mapping: detection must find the
// same input layout.
func TestGoldenInputs(t *testing.T) {
	golden.Run(t, golden.Dir(t), func(t *testing.T, c golden.Case) {
		m, err := config.LoadMapping(c.Path("mapping.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if plan, _ := engine.Compile(m); plan == nil {
			t.Skip("invalid configuration case")
		}
		res, err := File(c.Path("input.csv"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		in := m.Input

		wantEnc := in.Encoding
		if wantEnc == "latin-1" {
			wantEnc = "windows-1252" // superset; detection cannot tell them apart
		}
		check := func(what string, got, want any) {
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("%s = %v, want %v", what, got, want)
			}
		}
		check("encoding", res.Encoding, wantEnc)
		check("delimiter", res.Delimiter, config.Char(in.Delimiter))
		check("header", res.Header, *in.Header)
		check("header_row", res.HeaderRow, in.HeaderRow)
		check("skip_footer", res.SkipFooter, in.SkipFooter)
		check("trim", res.Trim, in.Trim)
	})
}

func detectString(t *testing.T, input string) *Result {
	t.Helper()
	res, err := Detect([]byte(input), nil, false, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func column(t *testing.T, res *Result, target string) Column {
	t.Helper()
	for _, c := range res.Columns {
		if c.Target == target {
			return c
		}
	}
	t.Fatalf("no column %q in %+v", target, res.Columns)
	return Column{}
}

func TestTypes(t *testing.T) {
	res := detectString(t, strings.Join([]string{
		"id;code;price;qty;active;flag;day;us_day;at;time;label",
		"1;007;1.234,50;1 000;oui;Y;13/01/2026;01/13/2026;2026-01-13T08:30:00Z;08:30;x",
		"2;010;12,5;12;non;n;02/02/2026;02/02/2026;2026-01-14T09:00:00+01:00;9:15;NA",
		"3;123;N/A;7;OUI;y;28/02/2026;02/28/2026;2026-01-15T10:00:00Z;23:59;z",
	}, "\n"))

	tests := []struct {
		target, typ string
		check       func(Column) string
	}{
		{"id", "integer", nil},
		{"code", "string", func(c Column) string { return expectNote(c, "leading zeros") }},
		{"price", "decimal", func(c Column) string {
			if c.DecimalSep != "," || c.ThousandsSep != "." || *c.Scale != 2 || !slices.Equal(c.NullTokens, []string{"N/A"}) {
				return fmt.Sprintf("dec=%q thousands=%q scale=%d nulls=%q", c.DecimalSep, c.ThousandsSep, *c.Scale, c.NullTokens)
			}
			return ""
		}},
		{"qty", "integer", func(c Column) string {
			if c.ThousandsSep != " " {
				return fmt.Sprintf("thousands = %q", c.ThousandsSep)
			}
			return ""
		}},
		{"active", "boolean", func(c Column) string {
			if !slices.Equal(c.TrueValues, []string{"oui"}) || !slices.Equal(c.FalseValues, []string{"non"}) {
				return fmt.Sprintf("true=%q false=%q", c.TrueValues, c.FalseValues)
			}
			return ""
		}},
		{"flag", "boolean", func(c Column) string {
			if c.TrueValues != nil {
				return "y/n is covered by default boolean values"
			}
			return ""
		}},
		{"day", "date", func(c Column) string { return expectFormats(c, "DD/MM/YYYY") }},
		{"us_day", "date", func(c Column) string { return expectFormats(c, "MM/DD/YYYY") }},
		{"at", "datetime", func(c Column) string { return expectFormats(c, "YYYY-MM-DDTHH:mm:ssZ") }},
		{"time", "time", func(c Column) string { return expectFormats(c, "HH:mm") }},
		{"label", "string", nil},
	}
	for _, tt := range tests {
		c := column(t, res, tt.target)
		if c.Type != tt.typ {
			t.Errorf("%s: type %s, want %s (notes %q)", tt.target, c.Type, tt.typ, c.Notes)
			continue
		}
		if tt.check != nil {
			if msg := tt.check(c); msg != "" {
				t.Errorf("%s: %s", tt.target, msg)
			}
		}
	}
	// N/A is read as empty everywhere: the text column holding "NA" is fine,
	// but the user is told when a token also appears in text.
	if !slices.Equal(res.NullTokens, []string{"N/A"}) {
		t.Errorf("null tokens = %q", res.NullTokens)
	}
}

func expectFormats(c Column, want ...string) string {
	if !slices.Equal(c.Formats, want) {
		return fmt.Sprintf("formats = %q, want %q", c.Formats, want)
	}
	return ""
}

func expectNote(c Column, substr string) string {
	for _, n := range c.Notes {
		if strings.Contains(n, substr) {
			return ""
		}
	}
	return fmt.Sprintf("no note containing %q in %q", substr, c.Notes)
}

func TestAmbiguousDayMonth(t *testing.T) {
	res := detectString(t, "d\n01/02/2026\n03/04/2026\n")
	c := column(t, res, "d")
	if msg := expectFormats(c, "DD/MM/YYYY"); msg != "" {
		t.Error(msg)
	}
	if msg := expectNote(c, "could be MM/DD/YYYY"); msg != "" {
		t.Error(msg)
	}
}

func TestEightDigitIntegersMayBeDates(t *testing.T) {
	c := column(t, detectString(t, "d\n20260105\n20261231\n"), "d")
	if c.Type != "integer" {
		t.Fatalf("type = %s", c.Type)
	}
	if msg := expectNote(c, "YYYYMMDD"); msg != "" {
		t.Error(msg)
	}
}

func TestNearMissNote(t *testing.T) {
	c := column(t, detectString(t, "d\n01/01/2026\n02/01/2026\n31/02/2026\n"), "d")
	if c.Type != "string" {
		t.Fatalf("type = %s", c.Type)
	}
	if msg := expectNote(c, "67% of values read as date DD/MM/YYYY; rejected: 31/02/2026"); msg != "" {
		t.Error(msg)
	}
}

func TestNoHeader(t *testing.T) {
	res := detectString(t, "1,2026-01-01,a\n2,2026-01-02,b\n3,2026-01-03,c\n")
	if res.Header || res.HeaderRow != 1 || res.SampleRows != 3 {
		t.Errorf("header=%v row=%d rows=%d", res.Header, res.HeaderRow, res.SampleRows)
	}
	if got := column(t, res, "column_2").Type; got != "date" {
		t.Errorf("column_2 type = %s", got)
	}
}

func TestAllTextAssumesHeader(t *testing.T) {
	res := detectString(t, "a,b\nc,d\n")
	if !res.Header {
		t.Error("all-text file: header expected")
	}
}

func TestUTF16WithoutBOM(t *testing.T) {
	var b []byte
	for _, r := range "id;name\n1;a\n2;b\n" {
		b = append(b, byte(r), 0)
	}
	res, err := Detect(b, nil, false, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Encoding != "utf-16le" || res.Delimiter != ";" || column(t, res, "id").Type != "integer" {
		t.Errorf("encoding=%s delimiter=%q", res.Encoding, res.Delimiter)
	}
}

// A file larger than the sample: the footer is found in the tail.
func TestLargeFileFooterInTail(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("id;amount\n")
	for i := range 5000 {
		fmt.Fprintf(&sb, "%d;%d,%02d\n", i, i, i%100)
	}
	sb.WriteString("Total;12345,00\nEnd of export\n")
	path := filepath.Join(t.TempDir(), "big.csv")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := File(path, Options{SampleBytes: 8 << 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.WholeFile || res.SkipFooter != 2 {
		t.Errorf("whole=%v footer=%d, want false and 2", res.WholeFile, res.SkipFooter)
	}
	if c := column(t, res, "amount"); c.Type != "decimal" || c.DecimalSep != "," {
		t.Errorf("amount: %s %q", c.Type, c.DecimalSep)
	}
}

func TestTargetName(t *testing.T) {
	tests := map[string]string{
		"N° Commande":      "n_commande",
		"Montant TTC (€)":  "montant_ttc",
		"Date de création": "date_de_creation",
		"  ":               "column_3",
		"2024":             "c_2024",
		"already_ok":       "already_ok",
	}
	for in, want := range tests {
		if got := TargetName(in, 3); got != want {
			t.Errorf("TargetName(%q) = %q, want %q", in, got, want)
		}
	}
}
