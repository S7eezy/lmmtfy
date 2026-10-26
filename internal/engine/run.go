package engine

import (
	"context"
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
	// Workers is the number of goroutines converting rows; 0 = one per CPU.
	Workers int
	// MaxRows stops after this many data rows (previews); 0 = all.
	MaxRows int
	// Context stops the conversion when cancelled (Ctrl+C); nil = never.
	Context context.Context
}

// Run converts job.Input into job.Output. The summary is always returned,
// with Status "failed" and the reason when err is not nil.
func (p *Plan) Run(job Job) (*report.Summary, error) {
	if job.Log == nil {
		job.Log = report.Discard
	}
	if job.Context == nil {
		job.Context = context.Background()
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
		if errors.Is(err, context.Canceled) {
			sum.FailureReason = "interrupted"
		}
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
	wopts     csvio.WriterOptions // with Numeric set for the active columns
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

	r.stats = map[string]*report.ColumnStats{}
	if err := r.pipeline(reader, first); err != nil {
		return err
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
	header, dup := NormalizeHeader(r.rawHeader)
	if dup != "" && r.plan.dupHeaders == "error" {
		return nil, &FileError{Reason: fmt.Sprintf("duplicate header %q (set input.duplicate_headers: suffix to number them)", dup)}
	}
	return header, nil
}

// NormalizeHeader trims header names and numbers duplicates: Name, Name_2,
// Name_3..., skipping names that already exist. dup is the first duplicated
// name, "" when there is none. Rules refer to columns by these names.
func NormalizeHeader(raw []string) (header []string, dup string) {
	header = make([]string, len(raw))
	trimmed := make([]string, len(raw))
	for i, h := range raw {
		trimmed[i] = strings.TrimSpace(h)
	}
	seen := map[string]int{}
	for i, h := range trimmed {
		seen[h]++
		if n := seen[h]; n > 1 {
			if dup == "" {
				dup = h
			}
			for ; ; n++ {
				candidate := fmt.Sprintf("%s_%d", h, n)
				if !slices.Contains(trimmed, candidate) && seen[candidate] == 0 {
					h = candidate
					seen[h]++
					break
				}
			}
		}
		header[i] = h
	}
	return header, dup
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
	r.writer, r.wopts = w, opts
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

// apply handles one computed row, in input order: statistics, logs, error
// strategies, reject file, output. It is the only stage touching shared
// state, so its effects are the same whatever the number of workers.
func (r *run) apply(res *rowResult, formatted []byte) error {
	rec := res.rec
	r.sum.RowsRead++
	log := r.job.Log
	if log.Enabled(report.LevelTrace) {
		log.Trace(report.Event{Line: rec.Line, Message: fmt.Sprintf("%q", rec.Fields)})
	}
	if res.rowErr != nil {
		return r.rowError(rec, []cellError{*res.rowErr})
	}

	var errs []cellError
	for _, e := range res.events {
		c := r.cols[e.col]
		switch e.kind {
		case eventWarning:
			r.stat(c.name).Warnings++
			log.Warn(report.Event{Line: rec.Line, Column: c.name, Message: e.msg})
		case eventFail:
			// Not logged as an event: the failure reason is in the summary.
			r.stat(c.name).Errors++
			r.sum.RowsWithErrors++
			return &FileError{Line: rec.Line, Column: c.name, Reason: e.msg}
		case eventDefaulted:
			s := r.stat(c.name)
			s.Errors++
			s.Defaulted++
			log.Error(report.Event{Line: rec.Line, Column: c.name, Message: e.msg + " (default used)"})
		case eventReject: // logged when the row is rejected
			r.stat(c.name).Errors++
			errs = append(errs, cellError{c.name, e.msg})
		}
	}
	if len(errs) > 0 {
		return r.reject(rec, errs)
	}
	if res.writeErr != nil {
		return r.rowError(rec, []cellError{*res.writeErr})
	}

	if err := r.writer.WriteFormatted(formatted); err != nil {
		return err
	}
	r.sum.RowsWritten++
	if res.defaulted {
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
