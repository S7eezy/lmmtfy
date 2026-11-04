package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/report"
)

// Saved tests: a mapping's examples, converted like a file and compared
// with their expected output. Test rows are raw text by source header,
// written as plain CSV (UTF-8, commas), so the plan reads them with the
// mapping's rules but without its file layout: no skipped lines, footer
// or comments.

// TestResult is the outcome of one saved test.
type TestResult struct {
	Name     string       `json:"name"`
	Passed   bool         `json:"passed"`
	Problems []string     `json:"problems,omitempty"` // why it failed, for people
	Output   []config.Row `json:"output"`             // the rows written
	Errors   []string     `json:"errors,omitempty"`   // "<column>: <message>"
}

// RunTests runs every saved test of the mapping, in order.
func (p *Plan) RunTests() []TestResult {
	results := make([]TestResult, len(p.Mapping.Tests))
	for i, t := range p.Mapping.Tests {
		results[i] = p.RunTest(t)
	}
	return results
}

var plainCSV = csvio.Dialect{Delimiter: ",", Quote: `"`, Escape: csvio.EscapeDouble}

// RunTest converts a test's input rows and compares the result with what
// it expects.
func (p *Plan) RunTest(t config.Test) TestResult {
	res := TestResult{Name: t.Name, Output: []config.Row{}}
	input, err := p.testInput(t.Input)
	if err != nil {
		res.Problems = []string{err.Error()}
		return res
	}

	tp := *p // the same rules, plain CSV in and out
	tp.reader = csvio.ReaderOptions{Dialect: plainCSV, Encoding: "utf-8", BOM: csvio.BOMAuto}
	tp.skipAfterHeader = 0
	tp.writer = csvio.WriterOptions{Dialect: plainCSV, Encoding: "utf-8", QuoteStyle: csvio.QuoteMinimal, LineEnding: "\n"}
	tp.outHeader = true

	var out, log bytes.Buffer
	_, runErr := tp.Run(Job{
		Input:   bytes.NewReader(input),
		Output:  &out,
		Log:     report.NewLogger(&log, report.LevelErrors, "json", nil),
		Workers: 1,
	})
	res.Errors = testErrors(&log, runErr)
	if runErr == nil {
		res.Output, err = testOutput(out.Bytes())
		if err != nil {
			res.Problems = append(res.Problems, "cannot read the output: "+err.Error())
		}
	}
	res.Problems = append(res.Problems, compareTest(t, res)...)
	res.Passed = len(res.Problems) == 0
	return res
}

// testInput writes test rows as a CSV file: the header (the columns of
// the rows, in order of appearance), then the rows. Without header, cells
// are named "#1", "#2"…
func (p *Plan) testInput(rows []config.Row) ([]byte, error) {
	var columns []string
	for _, row := range rows {
		for _, c := range row {
			if !slices.Contains(columns, c.Column) {
				columns = append(columns, c.Column)
			}
		}
	}
	if !p.header {
		width := 0
		for _, c := range columns {
			n, err := strconv.Atoi(strings.TrimPrefix(c, "#"))
			if err != nil || n < 1 {
				return nil, fmt.Errorf("the input has no header: name cells by position, like \"#1\", not %q", c)
			}
			width = max(width, n)
		}
		columns = make([]string, width)
		for i := range columns {
			columns[i] = "#" + strconv.Itoa(i+1)
		}
	}

	var buf bytes.Buffer
	w, err := csvio.NewWriter(&buf, csvio.WriterOptions{Dialect: plainCSV, Encoding: "utf-8", QuoteStyle: csvio.QuoteMinimal, LineEnding: "\n"})
	if err != nil {
		return nil, err
	}
	if p.header {
		if err := w.Write(columns); err != nil {
			return nil, err
		}
	}
	for _, row := range rows {
		fields := make([]string, len(columns))
		for i, c := range columns {
			fields[i], _ = row.Get(c)
		}
		if err := w.Write(fields); err != nil {
			return nil, err
		}
	}
	err = w.Flush()
	return buf.Bytes(), err
}

// testErrors lists the errors of a test run: logged errors, then the one
// that failed the file.
func testErrors(log io.Reader, runErr error) []string {
	var errs []string
	sc := bufio.NewScanner(log)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var e struct {
			Level, Column, Message string
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Level != "error" {
			continue
		}
		errs = append(errs, columnMessage(e.Column, e.Message))
	}
	var fe *FileError
	switch {
	case errors.As(runErr, &fe):
		errs = append(errs, columnMessage(fe.Column, fe.Reason))
	case runErr != nil:
		errs = append(errs, runErr.Error())
	}
	return errs
}

func columnMessage(column, msg string) string {
	if column == "" {
		return msg
	}
	return column + ": " + msg
}

// testOutput reads the rows of a test's output.
func testOutput(data []byte) ([]config.Row, error) {
	r, err := csvio.NewReader(bytes.NewReader(data), csvio.ReaderOptions{Dialect: plainCSV, Encoding: "utf-8", BOM: csvio.BOMAuto})
	if err != nil {
		return nil, err
	}
	rows := []config.Row{}
	var header []string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		if header == nil {
			header = rec.Fields
			continue
		}
		row := make(config.Row, len(header))
		for i, h := range header {
			row[i] = config.Cell{Column: h, Value: rec.Fields[i]}
		}
		rows = append(rows, row)
	}
}

// compareTest explains how a test's result differs from what it expects.
func compareTest(t config.Test, res TestResult) []string {
	var problems []string
	if len(res.Output) != len(t.Output) {
		problems = append(problems, fmt.Sprintf("expected %s, got %d", rowsText(len(t.Output)), len(res.Output)))
	}
	for i := range min(len(res.Output), len(t.Output)) {
		for _, want := range t.Output[i] {
			got, ok := res.Output[i].Get(want.Column)
			switch {
			case !ok:
				problems = append(problems, fmt.Sprintf("row %d, %s: not in the output", i+1, want.Column))
			case got != want.Value:
				problems = append(problems, fmt.Sprintf("row %d, %s: expected %q, got %q", i+1, want.Column, want.Value, got))
			}
		}
	}

	matched := make([]bool, len(res.Errors))
	for _, want := range t.Errors {
		found := false
		for i, got := range res.Errors {
			if strings.Contains(got, want) {
				found, matched[i] = true, true
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("expected an error mentioning %q", want))
		}
	}
	for i, got := range res.Errors {
		if !matched[i] {
			problems = append(problems, "unexpected error: "+got)
		}
	}
	return problems
}

func rowsText(n int) string {
	switch n {
	case 0:
		return "no row"
	case 1:
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}
