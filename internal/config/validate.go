package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// Severity of a configuration issue.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Issue is one problem found in a configuration. Path locates it, e.g.
// "columns.amount.decimal_sep" or "target.columns.amount.scale".
type Issue struct {
	Severity Severity
	Path     string
	Message  string
}

func (i Issue) String() string {
	if i.Path == "" {
		return fmt.Sprintf("%s: %s", i.Severity, i.Message)
	}
	return fmt.Sprintf("%s: %s: %s", i.Severity, i.Path, i.Message)
}

// Issues is a list of configuration issues.
type Issues []Issue

// Errorf adds an error.
func (is *Issues) Errorf(path, format string, args ...any) {
	*is = append(*is, Issue{SeverityError, path, fmt.Sprintf(format, args...)})
}

// Warnf adds a warning.
func (is *Issues) Warnf(path, format string, args ...any) {
	*is = append(*is, Issue{SeverityWarning, path, fmt.Sprintf(format, args...)})
}

// HasErrors reports whether at least one issue is an error.
func (is Issues) HasErrors() bool {
	return slices.ContainsFunc(is, func(i Issue) bool { return i.Severity == SeverityError })
}

// Err returns the errors as a single error, or nil when there is none.
func (is Issues) Err() error {
	var lines []string
	for _, i := range is {
		if i.Severity == SeverityError {
			lines = append(lines, i.String())
		}
	}
	if lines == nil {
		return nil
	}
	return errors.New("invalid configuration:\n  " + strings.Join(lines, "\n  "))
}

// Validate checks the structure of a mapping whose defaults have been
// applied. Type options and encodings are checked by the engine when it
// compiles the mapping, since they need the type and CSV implementations.
func Validate(m *Mapping) Issues {
	var is Issues

	if m.Version != Version {
		is.Errorf("version", "must be %d (got %d)", Version, m.Version)
	}
	if strings.TrimSpace(m.Name) == "" {
		is.Errorf("name", "required")
	}

	t := m.Target.Inline
	if t == nil {
		is.Errorf("target", "required")
		return is
	}
	validateTarget(&is, t)
	validateInput(&is, &m.Input)
	validateOnError(&is, &m.OnError)

	checkEnum(&is, "logging.level", m.Logging.Level, LogLevels)
	checkEnum(&is, "logging.format", m.Logging.Format, LogFormats)

	validateRules(&is, m)
	return is
}

func validateTarget(is *Issues, t *Target) {
	if t.Version != 0 && t.Version != Version {
		is.Errorf("target.version", "must be %d (got %d)", Version, t.Version)
	}

	o := t.Output
	checkChar(is, "target.output.delimiter", o.Delimiter, false)
	checkChar(is, "target.output.quote", o.Quote, true)
	if q := Char(o.Quote); q != "" && q == Char(o.Delimiter) {
		is.Errorf("target.output.quote", "must differ from the delimiter")
	}
	checkEnum(is, "target.output.quote_style", o.QuoteStyle, QuoteStyles)
	checkEnum(is, "target.output.escape", o.Escape, Escapes)
	checkEnum(is, "target.output.line_ending", o.LineEnding, LineEndings)

	if len(t.Columns) == 0 {
		is.Errorf("target.columns", "at least one column is required")
	}
	seen := map[string]bool{}
	for i, c := range t.Columns {
		path := fmt.Sprintf("target.columns[%d]", i)
		if strings.TrimSpace(c.Name) == "" {
			is.Errorf(path+".name", "required")
			continue
		}
		path = "target.columns." + c.Name
		if seen[c.Name] {
			is.Errorf(path, "duplicate column name")
		}
		seen[c.Name] = true
		checkEnum(is, path+".type", c.Type, Types)
	}
}

func validateInput(is *Issues, in *Input) {
	checkChar(is, "input.delimiter", in.Delimiter, false)
	checkChar(is, "input.quote", in.Quote, true)
	if q := Char(in.Quote); q != "" && q == Char(in.Delimiter) {
		is.Errorf("input.quote", "must differ from the delimiter")
	}
	checkEnum(is, "input.escape", in.Escape, Escapes)
	checkEnum(is, "input.bom", string(in.BOM), BOMModes)
	checkEnum(is, "input.ragged_rows", in.RaggedRows, RaggedRows)
	checkEnum(is, "input.duplicate_headers", in.DuplicateHeaders, DuplicateHeaders)
	if in.HeaderRow < 1 {
		is.Errorf("input.header_row", "must be 1 or more (1 = first line)")
	}
	if in.SkipRowsAfterHeader < 0 {
		is.Errorf("input.skip_rows_after_header", "must not be negative")
	}
	if in.SkipFooter < 0 {
		is.Errorf("input.skip_footer", "must not be negative")
	}
}

func validateOnError(is *Issues, e *OnError) {
	checkEnum(is, "on_error.strategy", e.Strategy, Strategies)
	if r := e.MaxErrorRate; r != nil && (*r < 0 || *r > 1) {
		is.Errorf("on_error.max_error_rate", "must be between 0 and 1 (0.01 = 1%%)")
	}
	if n := e.MaxErrors; n != nil && *n < 0 {
		is.Errorf("on_error.max_errors", "must not be negative")
	}
}

func validateRules(is *Issues, m *Mapping) {
	t := m.Target.Inline
	byName := map[string]TargetColumn{}
	for _, c := range t.Columns {
		byName[c.Name] = c
	}
	slots, slotIssues := m.Slots()
	*is = append(*is, slotIssues...)
	validateGroup(is, m, slots)

	// Rules naming no target column, sorted for stable output.
	var unknown []string
	for name := range m.Columns {
		if _, ok := byName[name]; !ok && !IsPattern(name) {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		is.Errorf("columns."+name, "no target column with this name")
	}

	for _, name := range slices.Sorted(maps.Keys(m.Helpers)) {
		path := "helpers." + name
		h := m.Helpers[name]
		switch {
		case strings.TrimSpace(name) == "":
			is.Errorf("helpers", "a helper needs a name")
			continue
		case slices.ContainsFunc(t.Columns, func(c TargetColumn) bool { return c.Name == name }):
			is.Errorf(path, "a target column has the same name: formulas could not tell them apart")
		}
		if h.Type == "" {
			is.Errorf(path+".type", "required")
		} else {
			checkEnum(is, path+".type", h.Type, Types)
		}
		if h.IfMissing == "omit" {
			is.Errorf(path+".if_missing", "omit only applies to output columns")
		}
		validateRule(is, path, h.Column(name), h.Rule, m)
	}

	checked := map[string]bool{}
	for _, c := range t.Columns {
		path := "columns." + c.Name
		r, ok := m.RuleOf(c.Name, slots)
		if s, slot := slots.Of[c.Name]; slot {
			// A pattern's rule is checked once, with its first column.
			if checked[s.Pattern] {
				continue
			}
			checked[s.Pattern] = true
			path = "columns." + s.Pattern
		}
		if !ok {
			switch {
			case c.Default != nil:
				is.Warnf(path, "not mapped: every row gets the target default %q", *c.Default)
			case c.Required:
				is.Errorf(path, "required target column is not mapped")
			default:
				is.Warnf(path, "not mapped: column left empty")
			}
			continue
		}
		validateRule(is, path, c, r, m)
	}
}

func validateRule(is *Issues, path string, c TargetColumn, r Rule, m *Mapping) {
	if r.Source != nil && r.SourceIndex != 0 {
		is.Errorf(path, "source and source_index are mutually exclusive")
	}
	if r.SourceIndex < 0 {
		is.Errorf(path+".source_index", "must be 1 or more (1 = first column)")
	}
	if r.Source != nil && !*m.Input.Header {
		is.Errorf(path+".source", "input has no header (input.header: false): use source_index")
	}

	checkEnum(is, path+".if_missing", r.IfMissing, IfMissingValues)
	checkEnum(is, path+".if_empty", r.IfEmpty, IfEmptyValues)
	checkEnum(is, path+".map_unmatched", r.MapUnmatched, MapUnmatched)
	if r.OnError != "" {
		checkEnum(is, path+".on_error", r.OnError, Strategies)
	}

	hasDefault := r.Default != nil || r.DefaultFormula != "" || c.Default != nil
	if r.Default != nil && r.DefaultFormula != "" {
		is.Errorf(path+".default_formula", "default and default_formula are mutually exclusive")
	}
	needDefault := func(field, why string) {
		if !hasDefault {
			is.Errorf(path+"."+field, "%s requires a default (on the rule or the target column)", why)
		}
	}
	if !r.HasSource() && !r.Computed() {
		needDefault("default", "a column without source or formula")
	}
	validateComputed(is, path, r)
	if r.IfMissing == "default" {
		needDefault("if_missing", "if_missing: default")
	}
	if r.IfEmpty == "default" {
		needDefault("if_empty", "if_empty: default")
	}
	if r.MapUnmatched == "default" {
		needDefault("map_unmatched", "map_unmatched: default")
	}
	if c.Required && r.EffectiveOnError(m) == "default" {
		needDefault("on_error", "on_error: default on a required column")
	}

	if r.Map == nil && r.MapUnmatched != "keep" {
		is.Errorf(path+".map_unmatched", "has no effect without map")
	}
	if c.Required && r.IfMissing == "empty" {
		is.Errorf(path+".if_missing", "a required column cannot be left empty when missing")
	}
	if r.IfMissing == "omit" && strings.HasPrefix(path, "columns.") {
		is.Warnf(path+".if_missing", "omit removes the column from the output when the source column is missing: the target software may reject files with a different column set")
	}
}

// validateComputed checks that a rule fills its column one way only.
func validateComputed(is *Issues, path string, r Rule) {
	ways := 0
	for _, used := range []bool{r.HasSource() || r.Formula != "", r.Cases != nil, r.Sequence != nil} {
		if used {
			ways++
		}
	}
	if ways > 1 {
		is.Errorf(path, "a column is filled one way: a source (with an optional formula), cases or a sequence")
	}

	if r.Cases != nil {
		if len(r.Cases) == 0 {
			is.Errorf(path+".cases", "at least one case is required")
		}
		for i, c := range r.Cases {
			p := fmt.Sprintf("%s.cases[%d]", path, i)
			if strings.TrimSpace(c.When) == "" {
				is.Errorf(p+".when", "required")
			}
			if (c.Then != nil) == (c.ThenFormula != "") {
				is.Errorf(p, "needs then or then_formula (one of them)")
			}
		}
	}
	if r.Cases == nil && (r.Else != nil || r.ElseFormula != "") {
		is.Errorf(path+".else", "has no effect without cases")
	}
	if r.Else != nil && r.ElseFormula != "" {
		is.Errorf(path+".else", "else and else_formula are mutually exclusive")
	}

	if s := r.Sequence; s != nil {
		if strings.HasPrefix(path, "helpers.") {
			is.Errorf(path+".sequence", "helpers are not written, so they have no row numbers: use row() in a formula")
		}
		if s.Step != nil && *s.Step == 0 {
			is.Errorf(path+".sequence.step", "must not be 0")
		}
	}
}

// EffectiveOnError is the rule's error strategy, falling back to the global one.
func (r Rule) EffectiveOnError(m *Mapping) string {
	if r.OnError != "" {
		return r.OnError
	}
	return m.OnError.Strategy
}

func checkEnum(is *Issues, path, value string, allowed []string) {
	if !slices.Contains(allowed, value) {
		is.Errorf(path, "%q is not one of: %s", value, strings.Join(allowed, ", "))
	}
}

// Character aliases accepted for delimiters and quotes, for characters that
// are awkward to write in YAML.
var charAliases = map[string]string{
	"comma":     ",",
	"semicolon": ";",
	"tab":       "\t",
	"pipe":      "|",
	"space":     " ",
	"none":      "",
}

// Char resolves an alias ("tab", "none", ...) to the character it stands for.
func Char(s Scalar) string {
	if c, ok := charAliases[string(s)]; ok {
		return c
	}
	return string(s)
}

func checkChar(is *Issues, path string, s Scalar, allowNone bool) {
	c := Char(s)
	switch {
	case c == "" && !allowNone:
		is.Errorf(path, "required")
	case c == "":
	case utf8.RuneCountInString(c) != 1:
		is.Errorf(path, "must be a single character (or an alias: comma, semicolon, tab, pipe, space), got %q", c)
	case c == "\r" || c == "\n":
		is.Errorf(path, "cannot be a line break")
	}
}
