package types

import (
	"errors"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
)

func s(v string) *config.Scalar { x := config.Scalar(v); return &x }
func n(v int) *int              { return &v }

type convCase struct {
	in      string
	want    string
	wantErr string // substring; "" = success
}

func runCases(t *testing.T, col config.TargetColumn, rule *config.Rule, cases []convCase) {
	t.Helper()
	conv, err := New(col, rule)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, c := range cases {
		got, err := conv.Convert(c.in)
		var w *Warning
		if errors.As(err, &w) {
			err = nil
		}
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("Convert(%q): unexpected error %v", c.in, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("Convert(%q) error = %v, want %q", c.in, err, c.wantErr)
		case c.wantErr == "" && got != c.want:
			t.Errorf("Convert(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	runCases(t, config.TargetColumn{Type: "string", Case: "upper", MaxLength: n(5), OnOverflow: "truncate"}, &config.Rule{}, []convCase{
		{"fr", "FR", ""},
		{"abcdefg", "ABCDE", ""},
		{"éèàùç", "ÉÈÀÙÇ", ""},
	})
	runCases(t, config.TargetColumn{Type: "string", MaxLength: n(3)}, &config.Rule{}, []convCase{
		{"abcd", "", "longer than 3"},
	})
	runCases(t, config.TargetColumn{Type: "string", Pattern: `[A-Z]{2}`}, &config.Rule{}, []convCase{
		{"FR", "FR", ""},
		{"FRA", "", "does not match pattern"}, // whole value must match
	})
	runCases(t, config.TargetColumn{Type: "string", AllowedValues: []config.Scalar{"A", "B"}}, &config.Rule{}, []convCase{
		{"A", "A", ""},
		{"C", "", "not an allowed value"},
	})
}

func TestTruncateWarns(t *testing.T) {
	conv, _ := New(config.TargetColumn{Type: "string", MaxLength: n(2), OnOverflow: "truncate"}, &config.Rule{})
	got, err := conv.Convert("abc")
	var w *Warning
	if got != "ab" || !errors.As(err, &w) {
		t.Errorf("got %q, %v; want ab and a Warning", got, err)
	}
}

func TestInteger(t *testing.T) {
	runCases(t, config.TargetColumn{Type: "integer", Min: s("0"), Max: s("1000000")}, &config.Rule{ThousandsSep: " "}, []convCase{
		{"42", "42", ""},
		{"+42", "42", ""},
		{"1 234", "1234", ""},
		{"007", "7", ""},
		{"-1", "", "less than the minimum"},
		{"12.5", "", "not an integer"},
		{"abc", "", "not an integer"},
		{"99999999999999999999", "", "out of the 64-bit"},
	})
	runCases(t, config.TargetColumn{Type: "integer", ThousandsSep: ","}, &config.Rule{}, []convCase{
		{"1234567", "1,234,567", ""},
		{"-1234", "-1,234", ""},
		{"123", "123", ""},
	})
}

func TestDecimal(t *testing.T) {
	runCases(t, config.TargetColumn{Type: "decimal", Scale: n(2)}, &config.Rule{DecimalSep: ",", ThousandsSep: " "}, []convCase{
		{"1 234,567", "1234.57", ""},
		{"0,5", "0.50", ""},
		{",5", "0.50", ""},
		{"-2,345", "-2.35", ""}, // half away from zero
		{"12", "12.00", ""},
		{"1.5", "", "decimal separator is"},
		{"1,2,3", "", "not a decimal"},
		{"1e5", "100000.00", ""},
		{"5,4e-03", "0.01", ""}, // decimal comma with exponent: 0.0054
		{"", "", "not a decimal"},
	})
	// Scientific notation, as written by Python, Excel or lab instruments.
	runCases(t, config.TargetColumn{Type: "decimal"}, &config.Rule{}, []convCase{
		{"5.419198916047157e-07", "0.0000005419198916047157", ""},
		{"1.5E+3", "1500", ""},
		{"-2.5e2", "-250", ""},
		{"1e308", "1" + strings.Repeat("0", 308), ""},
		{"1e309", "", "not a decimal"},   // beyond a double
		{"1e", "", "not a decimal"},      // no exponent digits
		{"e5", "", "not a decimal"},      // no mantissa
		{"1e5.5", "", "not a decimal"},   // fractional exponent
		{"1e+-5", "", "not a decimal"},   // two signs
		{"1.2.3e4", "", "not a decimal"}, // two dots
	})
	runCases(t, config.TargetColumn{Type: "decimal", Scale: n(2), Rounding: "half_even"}, &config.Rule{}, []convCase{
		{"2.345", "2.34", ""},
		{"2.355", "2.36", ""},
	})
	runCases(t, config.TargetColumn{Type: "decimal", Scale: n(1), Rounding: "truncate", DecimalSep: ",", ThousandsSep: "."}, &config.Rule{}, []convCase{
		{"1234567.89", "1.234.567,8", ""},
	})
	runCases(t, config.TargetColumn{Type: "decimal", Max: s("10")}, &config.Rule{}, []convCase{
		{"1.50", "1.5", ""}, // no scale: value as is, trailing zeros dropped
		{"10.01", "", "more than the maximum"},
	})
}

func TestBoolean(t *testing.T) {
	runCases(t, config.TargetColumn{Type: "boolean", TrueValue: s("1"), FalseValue: s("0")}, &config.Rule{}, []convCase{
		{"TRUE", "1", ""},
		{"y", "1", ""},
		{"No", "0", ""},
		{"maybe", "", "not a boolean"},
	})
	runCases(t, config.TargetColumn{Type: "boolean"}, &config.Rule{TrueValues: []config.Scalar{"oui"}, FalseValues: []config.Scalar{"non"}}, []convCase{
		{"OUI", "true", ""},
		{"non", "false", ""},
		{"yes", "", "not a boolean"},
	})
}

func TestTemporal(t *testing.T) {
	runCases(t, config.TargetColumn{Type: "date"}, &config.Rule{Formats: []string{"DD/MM/YYYY", "D-M-YY"}}, []convCase{
		{"31/12/2024", "2024-12-31", ""},
		{"5-3-24", "2024-03-05", ""},
		{"31/02/2024", "", "does not match any date format"},
		{"2024-12-31", "", "does not match any"},
	})
	runCases(t, config.TargetColumn{Type: "date", Format: "DD MMM YYYY"}, &config.Rule{Formats: []string{"YYYYMMDD"}}, []convCase{
		{"20240105", "05 Jan 2024", ""},
	})
	runCases(t, config.TargetColumn{Type: "datetime", Format: "YYYY-MM-DDTHH:mm:ssZ", Timezone: "UTC"},
		&config.Rule{Formats: []string{"DD/MM/YYYY HH:mm", "YYYY-MM-DDTHH:mm:ssZ"}, Timezone: "Europe/Paris"}, []convCase{
			{"15/07/2024 14:30", "2024-07-15T12:30:00Z", ""},          // summer: UTC+2
			{"15/01/2024 14:30", "2024-01-15T13:30:00Z", ""},          // winter: UTC+1
			{"2024-07-15T14:30:00+05:00", "2024-07-15T09:30:00Z", ""}, // own offset wins
		})
	runCases(t, config.TargetColumn{Type: "time", Format: "HH:mm"}, &config.Rule{Formats: []string{"h:mm A"}}, []convCase{
		{"2:05 PM", "14:05", ""},
	})
}

func TestDefaultsUseOutputRepresentation(t *testing.T) {
	conv, err := New(config.TargetColumn{Type: "decimal", Scale: n(2), DecimalSep: ","}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := conv.Convert("0,5"); got != "0,50" || err != nil {
		t.Errorf("default 0,5 = %q, %v", got, err)
	}
}

func TestOptionErrors(t *testing.T) {
	tests := []struct {
		col  config.TargetColumn
		rule config.Rule
		want string
	}{
		{config.TargetColumn{Type: "string", Scale: n(2)}, config.Rule{}, "scale: not valid for type string"},
		{config.TargetColumn{Type: "integer"}, config.Rule{Formats: []string{"YYYY"}}, "formats: not valid for type integer"},
		{config.TargetColumn{Type: "date", Timezone: "UTC"}, config.Rule{}, "timezone: not valid for type date"},
		{config.TargetColumn{Type: "date"}, config.Rule{Formats: []string{"DD/MM/2024"}}, "digits are not allowed"},
		{config.TargetColumn{Type: "date"}, config.Rule{Formats: []string{"DD/MM/YYYY hh:mm Q"}}, "unknown token"},
		{config.TargetColumn{Type: "datetime"}, config.Rule{Timezone: "Mars/Olympus"}, "unknown time zone"},
		{config.TargetColumn{Type: "decimal", DecimalSep: ","}, config.Rule{DecimalSep: ",", ThousandsSep: "."}, ""},
		{config.TargetColumn{Type: "decimal"}, config.Rule{DecimalSep: ",", ThousandsSep: ","}, "must differ from decimal_sep"},
		{config.TargetColumn{Type: "boolean", TrueValue: s("x"), FalseValue: s("x")}, config.Rule{}, "must differ"},
		{config.TargetColumn{Type: "string", Pattern: "("}, config.Rule{}, "invalid regular expression"},
	}
	for _, tt := range tests {
		_, err := New(tt.col, &tt.rule)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%+v / %+v: unexpected error %v", tt.col, tt.rule, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%+v / %+v: error = %v, want %q", tt.col, tt.rule, err, tt.want)
		}
	}
}

func TestLayout(t *testing.T) {
	tests := map[string]string{
		"YYYY-MM-DD":              "2006-01-02",
		"DD/MM/YY":                "02/01/06",
		"YYYY-MM-DDTHH:mm:ss.SSS": "2006-01-02T15:04:05.000",
		"D MMMM YYYY, h:mm a":     "2 January 2006, 3:04 pm",
		"YYYYMMDDHHmmss":          "20060102150405",
	}
	for format, want := range tests {
		got, err := Layout(format)
		if err != nil || got != want {
			t.Errorf("Layout(%q) = %q, %v; want %q", format, got, err, want)
		}
	}
	if _, err := Layout("HH:mm:ssSSS"); err == nil {
		t.Error("SSS without separator: want error")
	}
}
