package engine

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"lmmtfy/internal/csvio"
	"lmmtfy/internal/report"
	"lmmtfy/internal/types"
	"lmmtfy/internal/version"
)

// FileError means the input was rejected as a whole: fail_file strategy,
// error thresholds, missing required columns, malformed CSV.
type FileError struct {
	Line   int
	Column string
	Reason string
}

func (e *FileError) Error() string {
	switch {
	case e.Line > 0 && e.Column != "":
		return fmt.Sprintf("line %d, column %s: %s", e.Line, e.Column, e.Reason)
	case e.Line > 0:
		return fmt.Sprintf("line %d: %s", e.Line, e.Reason)
	}
	return e.Reason
}

// Job is one conversion, on streams.
type Job struct {
	Input  io.Reader
	Output io.Writer
	// Rejects opens the reject file. It is called on the first rejected row
	// only, so no reject file exists when nothing is rejected. Nil disables it.
	Rejects func() (io.Writer, error)
	Log     *report.Logger
}

// Run converts job.Input into job.Output. The summary is always returned,
// with Status "failed" and the reason when err is not nil.
func (p *Plan) Run(job Job) (*report.Summary, error) {
	if job.Log == nil {
		job.Log = report.Discard
	}
	start := time.Now()
	sum := &report.Summary{
		Mapping:       p.Mapping.Name,
		MappingSHA256: p.Hash,
		EngineVersion: version.Version,
		Status:        "ok",
	}
	r := &run{plan: p, job: job, sum: sum}
	err := r.run()
	sum.Columns = r.columnStats()
	sum.DurationSeconds = math.Round(time.Since(start).Seconds()*1000) / 1000
	if err != nil {
		sum.Status = "failed"
		sum.FailureReason = err.Error()
	}
	return sum, err
}

// active is a column resolved against one input file's header.
type active struct {
	*column
	idx      int  // field index in the input
	constant bool // no source in this file: every row gets constOut
	constOut string
}

type run struct {
	plan *Plan
	job  Job
	sum  *report.Summary

	rawHeader []string
	width     int
	cols      []*active
	out       []string
	writer    *csvio.Writer
	rejects   *csvio.Writer
	stats     map[string]*report.ColumnStats
}

func (r *run) run() error {
	p := r.plan
	reader, err := csvio.NewReader(r.job.Input, p.reader)
	if err != nil {
		return err
	}

	var first *csvio.Record
	rec, err := readRecord(reader)
	switch {
	case errors.Is(err, io.EOF) && p.header:
		return &FileError{Reason: "input is empty: no header row"}
	case errors.Is(err, io.EOF):
		// Headerless and empty: an empty output (header only) is correct.
	case err != nil:
		return err
	case p.header:
		r.rawHeader = rec.Fields
		r.width = len(rec.Fields)
	default:
		first = &rec
		r.width = len(rec.Fields)
	}

	header, err := r.normalizeHeader()
	if err != nil {
		return err
	}
	res := p.resolve(header, r.width)
	if len(res.missingRequired) > 0 {
		return &FileError{Reason: "required source columns missing from input: " + strings.Join(res.missingRequired, ", ")}
	}
	r.cols = res.active
	r.reportMissing(res.missing)

	if err := r.openWriter(); err != nil {
		return err
	}

	if p.header {
		for range p.skipAfterHeader {
			if _, err := readRecord(reader); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}
	}

	r.out = make([]string, len(r.cols))
	r.stats = map[string]*report.ColumnStats{}
	if first != nil {
		if err := r.row(*first); err != nil {
			return err
		}
	}
	for {
		rec, err := readRecord(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := r.row(rec); err != nil {
			return err
		}
	}

	if err := r.checkErrorRate(); err != nil {
		return err
	}
	if err := r.writer.Flush(); err != nil {
		return err
	}
	if r.rejects != nil {
		return r.rejects.Flush()
	}
	return nil
}

// readRecord turns CSV parse errors into file errors.
func readRecord(reader *csvio.Reader) (csvio.Record, error) {
	rec, err := reader.Read()
	var pe *csvio.ParseError
	if errors.As(err, &pe) {
		return rec, &FileError{Line: pe.Line, Reason: "malformed CSV: " + pe.Msg}
	}
	return rec, err
}

func (r *run) normalizeHeader() ([]string, error) {
	if r.rawHeader == nil {
		return nil, nil
	}
	header := make([]string, len(r.rawHeader))
	seen := map[string]int{}
	for i, h := range r.rawHeader {
		h = strings.TrimSpace(h)
		seen[h]++
		if n := seen[h]; n > 1 {
			if r.plan.dupHeaders == "error" {
				return nil, &FileError{Line: 0, Reason: fmt.Sprintf("duplicate header %q (set input.duplicate_headers: suffix to number them)", h)}
			}
			// Name, Name_2, Name_3... skipping names that already exist.
			for ; ; n++ {
				candidate := fmt.Sprintf("%s_%d", h, n)
				if !slices.Contains(r.rawHeader, candidate) && seen[candidate] == 0 {
					h = candidate
					seen[h]++
					break
				}
			}
		}
		header[i] = h
	}
	return header, nil
}

func (r *run) reportMissing(missing []report.MissingColumn) {
	if len(missing) == 0 {
		return
	}
	r.sum.MissingOptional = missing
	names := make([]string, len(missing))
	var detail strings.Builder
	for i, m := range missing {
		names[i] = m.Source
		fmt.Fprintf(&detail, "\n%s → %s: %s", m.Source, m.Target, missingAction(m, r.plan))
	}
	r.job.Log.Warn(report.Event{Message: fmt.Sprintf("configuration has %d optional columns missing from input: %s%s",
		len(missing), strings.Join(names, ", "), detail.String())})
}

func missingAction(m report.MissingColumn, p *Plan) string {
	switch m.Action {
	case "default":
		for _, c := range p.cols {
			if c.name == m.Target {
				return fmt.Sprintf("default %q", *c.defaultOut)
			}
		}
	case "omitted":
		return "omitted from output"
	}
	return "left empty"
}

func (r *run) openWriter() error {
	opts := r.plan.writer
	names := make([]string, len(r.cols))
	opts.Numeric = make([]bool, len(r.cols))
	for i, c := range r.cols {
		names[i] = c.name
		opts.Numeric[i] = c.numeric
	}
	w, err := csvio.NewWriter(r.job.Output, opts)
	if err != nil {
		return err
	}
	r.writer = w
	if r.plan.outHeader {
		if err := w.Write(names); err != nil {
			return &FileError{Reason: "cannot write output header: " + err.Error()}
		}
	}
	return nil
}

type cellError struct {
	column string
	msg    string
}

func (r *run) row(rec csvio.Record) error {
	p := r.plan
	r.sum.RowsRead++
	log := r.job.Log
	if log.Enabled(report.LevelTrace) {
		log.Trace(report.Event{Line: rec.Line, Message: fmt.Sprintf("%q", rec.Fields)})
	}

	fields := rec.Fields
	if n := len(fields); n != r.width {
		switch {
		case n < r.width && (p.ragged == "pad" || p.ragged == "lenient"):
			fields = append(slices.Clone(fields), make([]string, r.width-n)...)
		case n > r.width && (p.ragged == "truncate" || p.ragged == "lenient"):
		default:
			return r.rowError(rec, []cellError{{"", fmt.Sprintf("row has %d fields, expected %d", n, r.width)}})
		}
	}

	var errs []cellError
	defaulted := false
	for i, c := range r.cols {
		v, err := c.value(fields, p)
		if err == nil {
			r.out[i] = v
			continue
		}
		var warn *types.Warning
		if errors.As(err, &warn) {
			r.out[i] = v
			r.stat(c.name).Warnings++
			log.Warn(report.Event{Line: rec.Line, Column: c.name, Message: warn.Msg})
			continue
		}

		r.stat(c.name).Errors++
		switch c.onError {
		case "fail_file":
			// Not logged as an event: the failure reason is in the summary.
			r.sum.RowsWithErrors++
			return &FileError{Line: rec.Line, Column: c.name, Reason: err.Error()}
		case "default":
			r.out[i] = c.fallback(p)
			r.stat(c.name).Defaulted++
			defaulted = true
			log.Error(report.Event{Line: rec.Line, Column: c.name, Message: err.Error() + " (default used)"})
		default: // skip_row: logged when the row is rejected
			errs = append(errs, cellError{c.name, err.Error()})
		}
	}
	if len(errs) > 0 {
		return r.reject(rec, errs)
	}

	if err := r.writer.Write(r.out); err != nil {
		var re *csvio.RowError
		if !errors.As(err, &re) {
			return err
		}
		column := ""
		if re.Field >= 0 {
			column = r.cols[re.Field].name
		}
		return r.rowError(rec, []cellError{{column, re.Msg}})
	}
	r.sum.RowsWritten++
	if defaulted {
		r.sum.RowsWithErrors++
		return r.checkMaxErrors()
	}
	return nil
}

// rowError handles an error affecting the whole row, which no default can
// fix: the row fails the file or is rejected.
func (r *run) rowError(rec csvio.Record, errs []cellError) error {
	if r.plan.strategy == "fail_file" {
		e := errs[0]
		r.sum.RowsWithErrors++
		return &FileError{Line: rec.Line, Column: e.column, Reason: e.msg}
	}
	return r.reject(rec, errs)
}

func (r *run) reject(rec csvio.Record, errs []cellError) error {
	r.sum.RowsRejected++
	r.sum.RowsWithErrors++
	columns := make([]string, len(errs))
	msgs := make([]string, len(errs))
	for i, e := range errs {
		columns[i], msgs[i] = e.column, e.msg
		r.job.Log.Error(report.Event{Line: rec.Line, Column: e.column, Message: e.msg + " (row rejected)"})
	}

	if r.job.Rejects != nil {
		if r.rejects == nil {
			w, err := r.job.Rejects()
			if err != nil {
				return err
			}
			if r.rejects, err = csvio.NewWriter(w, r.plan.rejects); err != nil {
				return err
			}
			if r.rawHeader != nil {
				if err := r.rejects.Write(append(slices.Clone(r.rawHeader), "_line", "_column", "_error")); err != nil {
					return err
				}
			}
		}
		row := append(slices.Clone(rec.Fields), strconv.Itoa(rec.Line), strings.Join(columns, "; "), strings.Join(msgs, "; "))
		if err := r.rejects.Write(row); err != nil {
			return err
		}
	}
	return r.checkMaxErrors()
}

func (r *run) checkMaxErrors() error {
	if m := r.plan.maxErrors; m != nil && r.sum.RowsWithErrors > int64(*m) {
		return &FileError{Reason: fmt.Sprintf("more than %d rows with errors (on_error.max_errors)", *m)}
	}
	return nil
}

func (r *run) checkErrorRate() error {
	rate := r.plan.maxRate
	if rate == nil || r.sum.RowsRead == 0 {
		return nil
	}
	actual := float64(r.sum.RowsWithErrors) / float64(r.sum.RowsRead)
	if actual > *rate {
		return &FileError{Reason: fmt.Sprintf("%d of %d rows have errors (%.2f%%), more than the allowed %.2f%% (on_error.max_error_rate)",
			r.sum.RowsWithErrors, r.sum.RowsRead, actual*100, *rate*100)}
	}
	return nil
}

func (r *run) stat(name string) *report.ColumnStats {
	s, ok := r.stats[name]
	if !ok {
		s = &report.ColumnStats{Name: name}
		r.stats[name] = s
	}
	return s
}

// columnStats lists stats in output column order.
func (r *run) columnStats() []report.ColumnStats {
	var out []report.ColumnStats
	for _, c := range r.plan.cols {
		if s, ok := r.stats[c.name]; ok {
			out = append(out, *s)
		}
	}
	return out
}

// value runs the cell pipeline: trim, null tokens, value map, empty
// handling, type conversion.
func (a *active) value(fields []string, p *Plan) (string, error) {
	if a.constant {
		return a.constOut, nil
	}
	raw := fields[a.idx]
	if a.trim {
		raw = strings.TrimSpace(raw)
	}
	empty := raw == "" || p.nulls[raw]

	if !empty && a.vmap != nil {
		if to, ok := a.vmap[raw]; ok {
			raw = to
			empty = raw == ""
		} else {
			switch a.unmatched {
			case "error":
				return "", fmt.Errorf("%q has no entry in the value map", raw)
			case "default":
				return a.fallback(p), nil
			}
		}
	}

	if empty {
		switch a.ifEmpty {
		case "default":
			return *a.defaultOut, nil
		case "error":
			return "", errors.New("value is empty")
		}
		if a.required {
			return "", errors.New("required value is empty")
		}
		return p.nullOut, nil
	}
	return a.conv.Convert(raw)
}

// fallback is the value used instead of an error or a missing column.
func (c *column) fallback(p *Plan) string {
	if c.defaultOut != nil {
		return *c.defaultOut
	}
	return p.nullOut
}
