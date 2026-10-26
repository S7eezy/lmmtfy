package detect

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"lmmtfy/internal/config"
	"lmmtfy/internal/csvio"
	"lmmtfy/internal/types"
)

// Column is the proposal for one source column.
type Column struct {
	Index      int     `json:"index"`  // 1-based position
	Name       string  `json:"name"`   // header, "" without header
	Target     string  `json:"target"` // proposed target column name
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"` // share of non-empty values the type accepts
	Values     int     `json:"values"`     // non-empty values in the sample
	Empty      int     `json:"empty"`      // empty or null-token values in the sample

	// Parse options for the mapping rule.
	Formats      []string `json:"formats,omitempty"`
	DecimalSep   string   `json:"decimal_sep,omitempty"`
	ThousandsSep string   `json:"thousands_sep,omitempty"`
	TrueValues   []string `json:"true_values,omitempty"`
	FalseValues  []string `json:"false_values,omitempty"`
	// Scale is the largest number of decimals seen, for decimal columns.
	Scale *int `json:"scale,omitempty"`

	MaxLength  int      `json:"max_length"` // longest value, in characters
	NullTokens []string `json:"null_tokens,omitempty"`
	Examples   []string `json:"examples,omitempty"`
	Failing    []string `json:"failing,omitempty"` // values the type rejects
	Notes      []string `json:"notes,omitempty"`

	conv types.Converter
}

// Rule returns the mapping rule parse options for the column.
func (c Column) Rule() config.Rule {
	r := config.Rule{
		Formats:      c.Formats,
		DecimalSep:   config.Scalar(c.DecimalSep),
		ThousandsSep: config.Scalar(c.ThousandsSep),
	}
	for _, v := range c.TrueValues {
		r.TrueValues = append(r.TrueValues, config.Scalar(v))
	}
	for _, v := range c.FalseValues {
		r.FalseValues = append(r.FalseValues, config.Scalar(v))
	}
	return r
}

// TargetColumn returns the proposed target column: normalized output (ISO
// dates, "." decimals) that the user adapts to the target software.
func (c Column) TargetColumn() config.TargetColumn {
	return config.TargetColumn{Name: c.Target, Type: c.Type, Scale: c.Scale}
}

// Values read as empty when they appear in an otherwise typed column.
var commonNulls = []string{"NULL", "null", "Null", "N/A", "n/a", "NA", "#N/A", "-", "--", "?", "None", "none", "nil", "(null)"}

// candidate is one way to read a column.
type candidate struct {
	priority int // lower wins ties: integer before decimal before boolean...
	col      config.TargetColumn
	rule     config.Rule
	note     string // ambiguity worth telling the user, if chosen
}

// inferColumns proposes a type for each of width columns from data rows.
func inferColumns(data []csvio.Record, width int, trim bool, opts Options) []Column {
	cols := make([]Column, width)
	for i := range cols {
		var values []string
		empty := 0
		for _, r := range data {
			if i >= len(r.Fields) {
				continue
			}
			v := r.Fields[i]
			if trim {
				v = strings.TrimSpace(v)
			}
			if v == "" {
				empty++
				continue
			}
			values = append(values, v)
		}
		cols[i] = inferColumn(values, opts)
		cols[i].Index = i + 1
		cols[i].Empty += empty
	}
	return cols
}

func inferColumn(all []string, opts Options) Column {
	var values, nulls []string
	for _, v := range all {
		if slices.Contains(commonNulls, v) {
			if !slices.Contains(nulls, v) {
				nulls = append(nulls, v)
			}
			continue
		}
		values = append(values, v)
	}

	c := Column{Type: "string", Confidence: 1, Values: len(all)}
	for _, v := range all {
		c.MaxLength = max(c.MaxLength, utf8.RuneCountInString(v))
	}
	c.Examples = distinct(all, 3)
	if len(values) == 0 {
		c.conv, _ = types.New(config.TargetColumn{Type: "string"}, &config.Rule{})
		if len(all) == 0 {
			c.Notes = append(c.Notes, "no values in the sample")
		}
		return c
	}

	best, bestOK, bestFailing := candidate{priority: 1 << 30}, -1, []string(nil)
	maxFail := int(float64(len(values)) * (1 - opts.Threshold))
	probe := spread(values, 64)
	for _, cand := range candidates(values) {
		// Most readings fail on most columns: a quick try on a few values
		// spread over the sample avoids testing them on all of it.
		if ok, _ := evaluate(cand, probe, len(probe)/10); ok < 0 {
			continue
		}
		ok, failing := evaluate(cand, values, maxFail)
		if ok > bestOK || ok == bestOK && cand.priority < best.priority {
			best, bestOK, bestFailing = cand, ok, failing
		}
	}
	if bestOK < 0 {
		if cover, ok := coverDates(values, maxFail); ok {
			best, bestOK, bestFailing = cover, len(values)-len(failingOf(cover, values)), failingOf(cover, values)
		}
	}
	if bestOK < 0 {
		c.conv, _ = types.New(config.TargetColumn{Type: "string"}, &config.Rule{})
		if leadingZeros(values) {
			c.Notes = append(c.Notes, "numbers with leading zeros: kept as text (codes, identifiers)")
		}
		if note := nearMiss(values, opts); note != "" {
			c.Notes = append(c.Notes, note)
		}
		return c
	}

	c.Type = best.col.Type
	c.Empty = len(all) - len(values) // null tokens count as empty
	c.Values = len(values)
	c.Confidence = float64(bestOK) / float64(len(values))
	c.Formats = best.rule.Formats
	c.DecimalSep = string(best.rule.DecimalSep)
	c.ThousandsSep = string(best.rule.ThousandsSep)
	for _, v := range best.rule.TrueValues {
		c.TrueValues = append(c.TrueValues, string(v))
	}
	for _, v := range best.rule.FalseValues {
		c.FalseValues = append(c.FalseValues, string(v))
	}
	c.NullTokens = nulls
	c.Failing = distinct(bestFailing, 5)
	c.Examples = distinct(values, 3)
	if best.note != "" {
		c.Notes = append(c.Notes, best.note)
	}
	if c.Type == "date" || c.Type == "datetime" {
		for dayFirst, mf := range monthFirst {
			f := c.Formats[0]
			if len(c.Formats) == 1 && strings.HasPrefix(f, dayFirst) && matchesAll(values, c.Type, strings.Replace(f, dayFirst, mf, 1)) {
				c.Notes = append(c.Notes, fmt.Sprintf("day/month order is ambiguous: read as %s, could be %s", dayFirst, mf))
			}
		}
	}
	if c.Type == "decimal" {
		scale := maxDecimals(values, c.DecimalSep)
		c.Scale = &scale
	}
	if c.Type == "integer" && c.ThousandsSep == "" && matchesAll(values, "date", "YYYYMMDD") {
		c.Notes = append(c.Notes, "values also read as dates (YYYYMMDD): change the type if they are dates")
	}
	c.conv, _ = types.New(best.col, &best.rule)
	return c
}

// nearMiss explains a text column that is almost typed ("80% of values read
// as date DD/MM/YYYY; rejected: 31/02/2026"), so the user can fix the data
// or choose the type. Only the first values are tried, to stay fast.
func nearMiss(values []string, opts Options) string {
	values = spread(values, 100)
	best, bestOK, bestFailing := candidate{priority: 1 << 30}, -1, []string(nil)
	for _, cand := range candidates(values) {
		ok, failing := evaluate(cand, values, len(values)/2)
		if ok > bestOK || ok == bestOK && cand.priority < best.priority {
			best, bestOK, bestFailing = cand, ok, failing
		}
	}
	if bestOK <= 0 {
		return ""
	}
	desc := best.col.Type
	if len(best.rule.Formats) > 0 {
		desc += " " + best.rule.Formats[0]
	}
	return fmt.Sprintf("%.0f%% of values read as %s; rejected: %s", 100*float64(bestOK)/float64(len(values)), desc, strings.Join(distinct(bestFailing, 3), ", "))
}

// evaluate counts the values cand accepts. It gives up (returns -1) as soon
// as more than maxFail values fail.
func evaluate(cand candidate, values []string, maxFail int) (int, []string) {
	conv, err := types.New(cand.col, &cand.rule)
	if err != nil {
		return -1, nil
	}
	var failing []string
	for _, v := range values {
		if _, err := conv.Convert(v); err != nil {
			failing = append(failing, v)
			if len(failing) > maxFail {
				return -1, nil
			}
		}
	}
	return len(values) - len(failing), failing
}

func failingOf(cand candidate, values []string) []string {
	_, failing := evaluate(cand, values, len(values))
	return failing
}

// Thousands separators tried, with the decimal separators they can go with.
var groupings = []struct{ thousands, decimal string }{
	{",", "."}, {".", ","}, {" ", ","}, {" ", "."}, {" ", ","}, {" ", ","}, {"'", "."},
}

func candidates(values []string) []candidate {
	if leadingZeros(values) {
		// 007, 01234: identifiers, not numbers. Dates are still possible.
		return dateCandidates(values)
	}
	var out []candidate
	add := func(prio int, col config.TargetColumn, rule config.Rule, note string) {
		out = append(out, candidate{prio, col, rule, note})
	}

	add(0, config.TargetColumn{Type: "integer"}, config.Rule{}, "")
	add(1, config.TargetColumn{Type: "decimal"}, config.Rule{}, "")
	add(1, config.TargetColumn{Type: "decimal"}, config.Rule{DecimalSep: ","}, "")
	for _, g := range groupings {
		if !grouped(values, g.thousands, g.decimal) {
			continue
		}
		note := ""
		if g.thousands == "," || g.thousands == "." {
			note = fmt.Sprintf("%q read as a thousands separator; if the values are decimals, set decimal_sep: %q", g.thousands, g.thousands)
		}
		add(0, config.TargetColumn{Type: "integer"}, config.Rule{ThousandsSep: config.Scalar(g.thousands)}, note)
		add(1, config.TargetColumn{Type: "decimal"}, config.Rule{DecimalSep: config.Scalar(g.decimal), ThousandsSep: config.Scalar(g.thousands)}, "")
	}

	for _, pair := range booleanPairs {
		rule := config.Rule{}
		if !pair.defaults {
			rule.TrueValues = []config.Scalar{config.Scalar(pair.t)}
			rule.FalseValues = []config.Scalar{config.Scalar(pair.f)}
		}
		add(2, config.TargetColumn{Type: "boolean"}, rule, "")
	}
	return append(out, dateCandidates(values)...)
}

var booleanPairs = []struct {
	t, f     string
	defaults bool // covered by the boolean type's default true/false values
}{
	{"true", "false", true}, {"yes", "no", true}, {"y", "n", true}, {"t", "f", true},
	{"oui", "non", false}, {"vrai", "faux", false}, {"on", "off", false},
}

var (
	dateFormats = []string{
		"YYYY-MM-DD", "DD/MM/YYYY", "MM/DD/YYYY", "DD-MM-YYYY", "DD.MM.YYYY", "YYYY/MM/DD", "YYYYMMDD",
		"D/M/YYYY", "M/D/YYYY", "D.M.YYYY", "DD/MM/YY", "MM/DD/YY", "D/M/YY",
		"DD-MMM-YYYY", "D MMM YYYY", "DD MMM YYYY", "MMM D, YYYY", "D MMMM YYYY",
	}
	timeSuffixes = []string{
		" HH:mm:ss", " HH:mm", "THH:mm:ss", "THH:mm:ssZ", "THH:mm:ss.SSSZ", " HH:mm:ss.SSS", "THH:mm:ss.SSS", " HH:mm:ssZ",
	}
	timeFormats = []string{"HH:mm:ss", "HH:mm", "HH:mm:ss.SSS", "h:mm A", "h:mm:ss A"}
)

// Day-first formats come before month-first ones: when every day is 12 or
// less, both match, and the note tells the user.
var monthFirst = map[string]string{"DD/MM/YYYY": "MM/DD/YYYY", "D/M/YYYY": "M/D/YYYY", "DD/MM/YY": "MM/DD/YY"}

func dateCandidates(values []string) []candidate {
	if !slices.ContainsFunc(values, hasDigit) {
		return nil
	}
	var out []candidate
	for _, f := range dateFormats {
		out = append(out, candidate{3, config.TargetColumn{Type: "date"}, config.Rule{Formats: []string{f}}, ""})
	}
	for _, f := range dateFormats[:9] {
		for _, s := range timeSuffixes {
			out = append(out, candidate{4, config.TargetColumn{Type: "datetime"}, config.Rule{Formats: []string{f + s}}, ""})
		}
	}
	for _, f := range timeFormats {
		out = append(out, candidate{5, config.TargetColumn{Type: "time"}, config.Rule{Formats: []string{f}}, ""})
	}
	return out
}

// coverDates handles columns mixing date formats (05/10/2026 and
// 06-10-2026): it picks up to 3 formats that together accept enough values.
func coverDates(values []string, maxFail int) (candidate, bool) {
	probe := spread(values, 64)
	for _, kind := range []string{"date", "datetime"} {
		var formats []string
		for _, c := range dateCandidates(values) {
			// Only formats reading at least one of a few values are worth trying.
			if c.col.Type == kind && len(unmatched(probe, kind, c.rule.Formats[0])) < len(probe) {
				formats = append(formats, c.rule.Formats[0])
			}
		}
		remaining := values
		var chosen []string
		for len(chosen) < 3 && len(remaining) > maxFail {
			bestFormat, bestLeft := "", remaining
			for _, f := range formats {
				left := unmatched(remaining, kind, f)
				if len(left) < len(bestLeft) {
					bestFormat, bestLeft = f, left
				}
			}
			if bestFormat == "" {
				break
			}
			chosen = append(chosen, bestFormat)
			remaining = bestLeft
		}
		if len(chosen) > 1 && len(remaining) <= maxFail {
			return candidate{3, config.TargetColumn{Type: kind}, config.Rule{Formats: chosen}, "several formats in the same column"}, true
		}
	}
	return candidate{}, false
}

func unmatched(values []string, kind, format string) []string {
	conv, err := types.New(config.TargetColumn{Type: kind}, &config.Rule{Formats: []string{format}})
	if err != nil {
		return values
	}
	var left []string
	for _, v := range values {
		if _, err := conv.Convert(v); err != nil {
			left = append(left, v)
		}
	}
	return left
}

func matchesAll(values []string, kind, format string) bool {
	return len(unmatched(values, kind, format)) == 0
}

// grouped reports whether every value containing sep uses it as a
// thousands separator (1 234 567), and at least one value does.
func grouped(values []string, sep, dec string) bool {
	re := regexp.MustCompile(`^[+-]?\d{1,3}(` + regexp.QuoteMeta(sep) + `\d{3})+(` + regexp.QuoteMeta(dec) + `\d*)?$`)
	found := false
	for _, v := range values {
		if strings.Contains(v, sep) {
			if !re.MatchString(v) {
				return false
			}
			found = true
		}
	}
	return found
}

var leadingZero = regexp.MustCompile(`^[+-]?0\d+([.,]\d*)?$`)

func leadingZeros(values []string) bool {
	return slices.ContainsFunc(values, leadingZero.MatchString)
}

func maxDecimals(values []string, dec string) int {
	if dec == "" {
		dec = "."
	}
	n := 0
	for _, v := range values {
		if _, frac, ok := strings.Cut(v, dec); ok {
			n = max(n, len(frac))
		}
	}
	return n
}

// spread returns at most n values taken at regular intervals, so that a
// probe sees the whole sample, not just its first rows.
func spread(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	out := make([]string, n)
	for i := range out {
		out[i] = values[i*len(values)/n]
	}
	return out
}

func hasDigit(s string) bool { return strings.ContainsAny(s, "0123456789") }

func distinct(values []string, n int) []string {
	var out []string
	for _, v := range values {
		if len(out) == n {
			break
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
