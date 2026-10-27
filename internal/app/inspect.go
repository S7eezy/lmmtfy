package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/S7eezy/lmmtfy/internal/detect"
	"github.com/S7eezy/lmmtfy/internal/exitcode"
)

// Inspect detects the format of an input file and prints it, as text or
// JSON. It returns the process exit code.
func Inspect(name, input string, opts detect.Options, asJSON bool, stdout, stderr io.Writer) int {
	res, code := runDetect(name, input, opts, stderr)
	if res == nil {
		return code
	}
	if asJSON {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return exitcode.IO
		}
		stdout.Write(buf.Bytes())
		return exitcode.OK
	}
	io.WriteString(stdout, InspectText(input, res))
	return exitcode.OK
}

func runDetect(name, input string, opts detect.Options, stderr io.Writer) (*detect.Result, int) {
	if _, err := os.Stat(input); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return nil, exitcode.IO
	}
	res, err := detect.File(input, opts)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", name, input, err)
		return nil, exitcode.FileFailed
	}
	return res, exitcode.OK
}

// InspectText renders a detection result for humans.
func InspectText(input string, res *detect.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", input)
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	row := func(k, v string) { fmt.Fprintf(tw, "  %s\t%s\n", k, v) }

	enc := res.Encoding
	if res.BOM {
		enc += " (with BOM)"
	}
	row("encoding", enc)
	row("delimiter", delimiterName(res.Delimiter))
	if res.Header {
		row("header", fmt.Sprintf("line %d", res.HeaderRow))
	} else {
		row("header", fmt.Sprintf("none (data starts on line %d)", res.HeaderRow))
	}
	switch res.SkipFooter {
	case 0:
		row("footer", "none")
	case 1:
		row("footer", "1 line skipped")
	default:
		row("footer", fmt.Sprintf("%d lines skipped", res.SkipFooter))
	}
	row("trim", yesNo(res.Trim))
	if len(res.NullTokens) == 0 {
		row("null tokens", "none")
	} else {
		row("null tokens", quoteList(res.NullTokens))
	}
	if res.RaggedRows != "error" {
		row("ragged rows", res.RaggedRows)
	}
	coverage := "whole file"
	if !res.WholeFile {
		coverage = "start of the file"
	}
	row("sample", fmt.Sprintf("%d rows (%s)", res.SampleRows, coverage))
	tw.Flush()

	b.WriteString("\n")
	tw = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tcolumn\ttarget\ttype\tok\tdetails\texamples")
	var notes []string
	for _, c := range res.Columns {
		source := c.Name
		if source == "" {
			source = "-"
		}
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%.0f%%\t%s\t%s\n",
			c.Index, source, c.Target, c.Type, c.Confidence*100, details(c), quoteList(shortList(c.Examples)))
		label := c.Name
		if label == "" {
			label = fmt.Sprintf("#%d", c.Index)
		}
		if len(c.Failing) > 0 {
			notes = append(notes, fmt.Sprintf("%s: rejected values: %s", label, quoteList(c.Failing)))
		}
		for _, n := range c.Notes {
			notes = append(notes, label+": "+n)
		}
	}
	tw.Flush()

	notes = append(notes, res.Notes...)
	if len(notes) > 0 {
		b.WriteString("\nnotes:\n")
		for _, n := range notes {
			fmt.Fprintf(&b, "  - %s\n", n)
		}
	}
	return b.String()
}

func details(c detect.Column) string {
	var parts []string
	switch c.Type {
	case "date", "datetime", "time":
		parts = append(parts, "formats "+strings.Join(c.Formats, ", "))
	case "decimal":
		dec := c.DecimalSep
		if dec == "" {
			dec = "."
		}
		parts = append(parts, fmt.Sprintf("decimal %q", dec))
		if c.ThousandsSep != "" {
			parts = append(parts, fmt.Sprintf("thousands %q", c.ThousandsSep))
		}
		if c.Scale != nil {
			parts = append(parts, fmt.Sprintf("scale %d", *c.Scale))
		}
	case "integer":
		if c.ThousandsSep != "" {
			parts = append(parts, fmt.Sprintf("thousands %q", c.ThousandsSep))
		}
	case "boolean":
		if c.TrueValues != nil {
			parts = append(parts, fmt.Sprintf("true %s, false %s", quoteList(c.TrueValues), quoteList(c.FalseValues)))
		}
	case "string":
		if c.MaxLength == 1 {
			parts = append(parts, "max 1 char")
		} else {
			parts = append(parts, fmt.Sprintf("max %d chars", c.MaxLength))
		}
	}
	if c.Empty > 0 {
		parts = append(parts, fmt.Sprintf("%d empty", c.Empty))
	}
	return strings.Join(parts, ", ")
}

func delimiterName(d string) string {
	switch d {
	case "\t":
		return "tab"
	case " ":
		return "space"
	}
	return fmt.Sprintf("%q", d)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func quoteList(values []string) string {
	q := make([]string, len(values))
	for i, v := range values {
		q[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(q, ", ")
}

func shortList(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		if r := []rune(v); len(r) > 24 {
			v = string(r[:24]) + "…"
		}
		out[i] = v
	}
	return out
}
