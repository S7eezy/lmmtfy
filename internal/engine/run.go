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

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/expr"
	"github.com/S7eezy/lmmtfy/internal/report"
	"github.com/S7eezy/lmmtfy/internal/types"
	"github.com/S7eezy/lmmtfy/internal/version"
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
	// Trace, when set, records how one column is filled (previews).
	Trace *Trace
	// AggregateAll computes aggregates over the whole input even with
	// MaxRows, which otherwise limits them too.
	AggregateAll bool
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
	r := &run{plan: p, job: job, sum: sum, start: start}
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
	idx        int  // field index in the input
	pos        int  // index in the output row, -1 for helpers
	constant   bool // no source in this file: every row gets constOut
	constOut   string
	constEmpty bool // constOut is an empty value
	absent     bool // source missing, left empty: the formula runs on an empty value
	// constDefault: constOut is the default, known once the file's
	// default formulas are computed.
	constDefault bool
	defaultOut   *string // this file's default; shadows column.defaultOut
}

type run struct {
	plan  *Plan
	job   Job
	sum   *report.Summary
	start time.Time

	rawHeader []string
	width     int
	cols      []*active // output columns
	order     []*active // every column, in evaluation order
	traced    *active   // the column of job.Trace
	numbered  bool      // an output column is a sequence: apply formats rows
	numberer  *csvio.Formatter
	numberBuf []byte

	byIdx   []int // fields of group.by, when rows are combined
	helpers []int // ids of the helpers of this input

	inputStart int64               // offset of the input when the job started
	aggregates []expr.Value        // by slot, from the first pass
	wopts      csvio.WriterOptions // with Numeric set for the active columns
	writer     *csvio.Writer
	rejects    *csvio.Writer
	stats      map[string]*report.ColumnStats
}

func (r *run) run() error {
	p := r.plan
	if len(p.aggregates) > 0 && p.group == nil {
		cleanup, err := r.rereadable()
		if err != nil {
			return err
		}
		defer cleanup()
	}
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
	r.cols, r.order = res.active, res.order
	for _, a := range r.order {
		if a.helper {
			r.helpers = append(r.helpers, a.id)
		}
	}
	if g := p.group; g != nil {
		for _, name := range g.by {
			i := slices.Index(header, name)
			if i < 0 {
				return &FileError{Reason: fmt.Sprintf("group.by: source column %q is not in the input", name)}
			}
			r.byIdx = append(r.byIdx, i)
		}
	}
	r.numbered = slices.ContainsFunc(r.cols, func(a *active) bool { return a.seq != nil })
	if t := r.job.Trace; t != nil {
		for _, a := range r.order {
			if a.name == t.Column {
				r.traced = a
			}
		}
	}
	if len(p.aggregates) > 0 && p.group == nil {
		if err := r.firstPass(); err != nil {
			return err
		}
	}
	if err := r.defaults(); err != nil {
		return err
	}
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
		fmt.Fprintf(&detail, "\n%s → %s: %s", m.Source, m.Target, r.missingAction(m))
	}
	r.job.Log.Warn(report.Event{Message: fmt.Sprintf("configuration has %d optional columns missing from input: %s%s",
		len(missing), strings.Join(names, ", "), detail.String())})
}

func (r *run) missingAction(m report.MissingColumn) string {
	switch m.Action {
	case "default":
		for _, a := range r.order {
			if a.name == m.Target {
				return fmt.Sprintf("default %q", a.fallback(r.plan))
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
	rows := res.recs // a batch's rows, when rows are combined
	if rows != nil {
		r.sum.RowsRead += int64(len(rows))
		r.sum.Batches++
	} else {
		rows = []csvio.Record{rec}
		r.sum.RowsRead++
	}
	if t := res.trace; t != nil && len(r.job.Trace.Rows) < r.job.Trace.Max {
		t.Line = rec.Line
		r.job.Trace.Rows = append(r.job.Trace.Rows, *t)
	}
	log := r.job.Log
	if log.Enabled(report.LevelTrace) {
		log.Trace(report.Event{Line: rec.Line, Message: fmt.Sprintf("%q", rec.Fields)})
	}
	if res.rowErr != nil {
		return r.rowError(rows, []cellError{*res.rowErr})
	}

	var errs []cellError
	for _, e := range res.events {
		c := e.col
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
		return r.reject(rows, errs)
	}
	if res.writeErr != nil {
		return r.rowError(rows, []cellError{*res.writeErr})
	}
	if res.cells != nil {
		var cellErr *cellError
		if formatted, cellErr = r.number(res.cells); cellErr != nil {
			return r.rowError(rows, []cellError{*cellErr})
		}
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

// number fills the sequence columns of a row about to be written, then
// formats it. Numbers follow written rows: a rejected row takes none.
func (r *run) number(cells []string) ([]byte, *cellError) {
	n := r.sum.RowsWritten
	for _, c := range r.cols {
		if c.seq == nil {
			continue
		}
		out, err := c.format.Convert(strconv.FormatInt(c.seq.start+c.seq.step*n, 10))
		if err != nil && !errors.As(err, new(*types.Warning)) {
			return nil, &cellError{c.name, err.Error()}
		}
		cells[c.pos] = out
	}
	if r.numberer == nil {
		r.numberer, _ = csvio.NewFormatter(r.wopts) // options already checked by openWriter
	}
	b, err := r.numberer.Append(r.numberBuf[:0], cells)
	if err != nil {
		var re *csvio.RowError
		if !errors.As(err, &re) {
			re = &csvio.RowError{Field: -1, Msg: err.Error()}
		}
		column := ""
		if re.Field >= 0 {
			column = r.cols[re.Field].name
		}
		return nil, &cellError{column, re.Msg}
	}
	r.numberBuf = b
	return b, nil
}

// rowError handles an error affecting the whole row, which no default can
// fix: the row fails the file or is rejected.
func (r *run) rowError(rows []csvio.Record, errs []cellError) error {
	rec := rows[0]
	if r.plan.strategy == "fail_file" {
		e := errs[0]
		r.sum.RowsWithErrors++
		return &FileError{Line: rec.Line, Column: e.column, Reason: e.msg}
	}
	return r.reject(rows, errs)
}

func (r *run) reject(rows []csvio.Record, errs []cellError) error {
	rec := rows[0]
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
		// Every row of a rejected batch, so the reject file can be fixed
		// and converted again.
		for _, rec := range rows {
			row := append(slices.Clone(rec.Fields), strconv.Itoa(rec.Line), strings.Join(columns, "; "), strings.Join(msgs, "; "))
			if err := r.rejects.Write(row); err != nil {
				return err
			}
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
	total, what := r.sum.RowsRead, "rows"
	if r.plan.group != nil {
		total, what = r.sum.Batches, "batches" // errors count output rows
	}
	actual := float64(r.sum.RowsWithErrors) / float64(total)
	if actual > *rate {
		return &FileError{Reason: fmt.Sprintf("%d of %d %s have errors (%.2f%%), more than the allowed %.2f%% (on_error.max_error_rate)",
			r.sum.RowsWithErrors, total, what, actual*100, *rate*100)}
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
	for _, c := range r.plan.nodes {
		if s, ok := r.stats[c.name]; ok {
			out = append(out, *s)
		}
	}
	return out
}

// value runs the cell pipeline: trim, null tokens, value map, empty
// handling, type conversion. empty reports an empty value. It is the hot
// path: computed shares its steps through raw, which this inlines.
func (a *active) value(fields []string, p *Plan) (out string, empty bool, err error) {
	if a.constant {
		return a.constOut, a.constEmpty, nil
	}
	raw := fields[a.idx]
	if a.trim {
		raw = strings.TrimSpace(raw)
	}
	isEmpty := raw == "" || p.nulls[raw]

	if !isEmpty && a.vmap != nil {
		if to, ok := a.vmap[raw]; ok {
			raw = to
			isEmpty = raw == ""
		} else {
			switch a.unmatched {
			case "error":
				return "", false, fmt.Errorf("%q has no entry in the value map", raw)
			case "default":
				return a.fallback(p), a.fallbackEmpty(), nil
			}
		}
	}

	if isEmpty {
		switch a.ifEmpty {
		case "default":
			return a.fallback(p), a.fallbackEmpty(), nil
		case "error":
			return "", false, errors.New("value is empty")
		}
		if a.required {
			return "", false, errors.New("required value is empty")
		}
		return p.nullOut, true, nil
	}
	out, err = a.conv.Convert(raw)
	return out, false, err
}

// raw extracts the source value of a row, after trimming, null tokens, the
// value map and empty handling. final is true when that settled the output
// (a default, an error); otherwise raw is the value to convert, "" when
// empty.
func (a *active) raw(fields []string, p *Plan) (raw string, final bool, out string, empty bool, err error) {
	raw = fields[a.idx]
	if a.trim {
		raw = strings.TrimSpace(raw)
	}
	isEmpty := raw == "" || p.nulls[raw]

	if !isEmpty && a.vmap != nil {
		if to, ok := a.vmap[raw]; ok {
			raw = to
			isEmpty = raw == ""
		} else {
			switch a.unmatched {
			case "error":
				return "", true, "", false, fmt.Errorf("%q has no entry in the value map", raw)
			case "default":
				return "", true, a.fallback(p), a.fallbackEmpty(), nil
			}
		}
	}

	if isEmpty {
		switch a.ifEmpty {
		case "default":
			return "", true, a.fallback(p), a.fallbackEmpty(), nil
		case "error":
			return "", true, "", false, errors.New("value is empty")
		}
		return "", false, "", false, nil
	}
	return raw, false, "", false, nil
}

// cased is value for a column with cases: the first true condition gives
// the value, else the else value, else empty.
func (a *active) cased(p *Plan, env *expr.Env) (out string, empty bool, err error) {
	cs := a.cases
	for i, when := range cs.when {
		v, err := when.Eval(env)
		if err != nil {
			return "", false, err
		}
		if v.IsTrue() {
			return a.outcome(cs.then[i], p, env)
		}
	}
	if cs.otherwise != nil {
		return a.outcome(*cs.otherwise, p, env)
	}
	if a.required {
		return "", false, errors.New("required value is empty: no case matches, and there is no else")
	}
	return p.nullOut, true, nil
}

func (a *active) outcome(o outcome, p *Plan, env *expr.Env) (string, bool, error) {
	if o.prog == nil {
		return o.out, o.empty, nil
	}
	return a.result(o.prog, p, env)
}

// result evaluates a formula and writes its result.
func (a *active) result(prog *expr.Program, p *Plan, env *expr.Env) (out string, empty bool, err error) {
	v, err := prog.Eval(env)
	if err != nil {
		return "", false, err
	}
	if v.IsEmpty() {
		if a.required {
			return "", false, errors.New("required value is empty: the formula gave nothing")
		}
		return p.nullOut, true, nil
	}
	out, err = a.format.Convert(expr.Convert(v, a.typ).Canonical())
	return out, false, err
}

// computed is value for a column with a formula. Defaults are final: the
// formula runs on the parsed source value, an empty one included, or with
// no value at all for a column without source.
func (a *active) computed(fields []string, p *Plan, env *expr.Env) (out string, empty bool, err error) {
	if a.constant {
		return a.constOut, a.constEmpty, nil
	}
	env.Value = expr.Value{}
	if a.hasSource && !a.absent {
		raw, final, out, empty, err := a.raw(fields, p)
		if final || err != nil {
			return out, empty, err
		}
		if raw != "" {
			canon, err := a.parse.Convert(raw)
			if err != nil {
				return "", false, err
			}
			if env.Value, err = expr.Parse(a.typ, canon); err != nil {
				return "", false, err
			}
		}
	}

	return a.result(a.formula, p, env)
}

// fallback is the value used instead of an error or a missing column.
func (a *active) fallback(p *Plan) string {
	if a.defaultOut != nil {
		return *a.defaultOut
	}
	return p.nullOut
}

func (a *active) fallbackEmpty() bool { return a.defaultOut == nil || *a.defaultOut == "" }
