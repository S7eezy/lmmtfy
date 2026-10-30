package detect

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/engine"
)

// detectFooter counts trailing records that do not fit the columns: totals,
// "end of file" markers. At most 3. Types are inferred without the last
// records first, since a footer would spoil them.
func detectFooter(data []csvio.Record, tail []byte, truncated bool, width int, res *Result, opts Options) int {
	var base, last []csvio.Record
	if truncated {
		base = data
		last = parse(tail, res, 1<<30)
	} else {
		k := min(3, len(data)-1)
		if k <= 0 {
			return 0
		}
		base, last = data[:len(data)-k], data
	}
	cols := inferColumns(base, width, res.Trim, opts)

	footer := 0
	for i := len(last) - 1; i >= 0 && footer < 3; i-- {
		if !misfit(last[i], cols, width, res.Trim) {
			break
		}
		footer++
	}
	// Everything misfitting means the types are wrong, not that there is a footer.
	if footer > 0 && footer == len(last) {
		return 0
	}
	return footer
}

// misfit reports whether a record looks like a footer rather than data:
// a short row with few values ("End of export"), values most typed columns
// reject ("Total;3;;;" under integer and date columns), or a rejected value
// in a row starting with a word like "Total". A single bad value is a data
// error, not a footer; long rows are never footers.
func misfit(rec csvio.Record, cols []Column, width int, trim bool) bool {
	if len(rec.Fields) > width {
		return false
	}
	if len(rec.Fields) < width {
		return filledCount(rec.Fields)*2 <= width
	}
	checked, failed := 0, 0
	for i, c := range cols {
		if c.Type == "string" {
			continue
		}
		v := rec.Fields[i]
		if trim {
			v = strings.TrimSpace(v)
		}
		if v == "" || slices.Contains(c.NullTokens, v) {
			continue
		}
		checked++
		if _, err := c.conv.Convert(v); err != nil {
			failed++
		}
	}
	return checked >= 2 && failed*2 > checked || failed > 0 && footerWord.MatchString(firstValue(rec.Fields))
}

var footerWord = regexp.MustCompile(`(?i)^(total|totaux|sum|somme|count|nombre|end|fin)\b`)

func firstValue(fields []string) string {
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			return f
		}
	}
	return ""
}

func filledCount(fields []string) int {
	n := 0
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			n++
		}
	}
	return n
}

// looksLikeData reports whether the candidate header row parses like the
// rows below it, in most typed columns.
func looksLikeData(rec csvio.Record, cols []Column, trim bool) bool {
	typed, parsed := 0, 0
	for i, c := range cols {
		if c.Type == "string" || i >= len(rec.Fields) {
			continue
		}
		typed++
		v := rec.Fields[i]
		if trim {
			v = strings.TrimSpace(v)
		}
		if _, err := c.conv.Convert(v); err == nil {
			parsed++
		}
	}
	return parsed > 0 && parsed*2 >= typed
}

// finalColumns infers types on the final data rows, names the columns and
// collects null tokens.
func finalColumns(header []string, data []csvio.Record, width int, res *Result, opts Options) []Column {
	cols := inferColumns(data, width, res.Trim, opts)
	// Names exactly as the engine will see them, duplicates numbered, so
	// rules generated from them find their column.
	names, _ := engine.NormalizeHeader(header)
	used := map[string]bool{}
	for i := range cols {
		c := &cols[i]
		if i < len(names) {
			c.Name = names[i]
		}
		c.Target = uniqueName(TargetName(c.Name, i+1), used)
		for _, tok := range c.NullTokens {
			if !slices.Contains(res.NullTokens, tok) {
				res.NullTokens = append(res.NullTokens, tok)
			}
		}
	}
	// Null tokens apply to every column: warn where they hide real text.
	for _, tok := range res.NullTokens {
		for _, c := range cols {
			if c.Type == "string" && slices.Contains(c.Examples, tok) {
				res.Notes = append(res.Notes, fmt.Sprintf("null token %q also appears in text column %s, where it will be read as empty", tok, c.label()))
			}
		}
	}
	return cols
}

func (c Column) label() string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("#%d", c.Index)
}

// TargetName turns a header into a target column name: ASCII letters,
// digits and underscores, case kept ("N° Commande" → "N_Commande").
// Columns without a usable name become column_<index>.
func TargetName(header string, index int) string {
	// Decompose "é" into "e" + accent, then drop the accent.
	stripMarks := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	plain, _, err := transform.String(stripMarks, header)
	if err != nil {
		plain = header
	}
	var b bytes.Buffer
	underscore := false
	for _, r := range plain {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			if underscore && b.Len() > 0 {
				b.WriteByte('_')
			}
			underscore = false
			b.WriteRune(r)
		default:
			underscore = true
		}
	}
	name := b.String()
	switch {
	case name == "":
		return fmt.Sprintf("column_%d", index)
	case name[0] >= '0' && name[0] <= '9':
		return "c_" + name
	}
	return name
}

// FileName turns a name into a file-friendly one: TargetName in lower
// case ("Orders 2026.csv" → "orders_2026_csv").
func FileName(name string) string {
	return strings.ToLower(TargetName(name, 1))
}

func uniqueName(name string, used map[string]bool) string {
	candidate := name
	for n := 2; used[candidate]; n++ {
		candidate = fmt.Sprintf("%s_%d", name, n)
	}
	used[candidate] = true
	return candidate
}
