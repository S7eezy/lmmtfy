package types

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
)

// Converter turns a source cell into its output representation: parse with
// the mapping's input options, validate against the target constraints,
// format with the target's output options.
type Converter interface {
	Convert(raw string) (string, error)
}

// Warning is returned by Convert when the value was converted but altered
// (e.g. truncated). The returned string is valid and should be used.
type Warning struct{ Msg string }

func (w *Warning) Error() string { return w.Msg }

// New builds the converter for a target column. rule holds the source parse
// options; nil means the input is already in the output representation,
// which is how defaults are checked and converted.
func New(col config.TargetColumn, rule *config.Rule) (Converter, error) {
	in := config.Rule{}
	if rule != nil {
		in = *rule
	}
	o := &opts{col: col, rule: in, fromOutput: rule == nil}

	var conv Converter
	switch col.Type {
	case "string":
		conv = newString(o)
	case "integer":
		conv = newInteger(o)
	case "decimal":
		conv = newDecimal(o)
	case "boolean":
		conv = newBoolean(o)
	case "date", "datetime", "time":
		conv = newTemporal(o)
	default:
		return nil, fmt.Errorf("type: unknown type %q", col.Type)
	}
	o.checkUnused()
	if err := errors.Join(o.errs...); err != nil {
		return nil, err
	}
	return conv, nil
}

// opts tracks which options the type builder used, so that options set but
// meaningless for the type are reported instead of silently ignored.
type opts struct {
	col        config.TargetColumn
	rule       config.Rule
	fromOutput bool
	used       map[string]bool
	errs       []error
}

func (o *opts) use(names ...string) {
	if o.used == nil {
		o.used = map[string]bool{}
	}
	for _, n := range names {
		o.used[n] = true
	}
}

// errorf records an error about an option. Target options are prefixed with
// "target:" so the caller can place the message on the right path.
func (o *opts) errorf(option, format string, args ...any) {
	o.errs = append(o.errs, &OptionError{Option: option, Msg: fmt.Sprintf(format, args...)})
}

// OptionError is a problem with one type option.
type OptionError struct {
	Option string // "target:scale" for target options, "formats" for rule options
	Msg    string
}

func (e *OptionError) Error() string {
	return strings.TrimPrefix(e.Option, "target:") + ": " + e.Msg
}

// IsTarget reports whether the option belongs to the target column.
func (e *OptionError) IsTarget() bool { return strings.HasPrefix(e.Option, "target:") }

func (o *opts) checkUnused() {
	c, r := o.col, o.rule
	set := map[string]bool{
		"target:max_length":     c.MaxLength != nil,
		"target:on_overflow":    c.OnOverflow != "",
		"target:case":           c.Case != "",
		"target:pattern":        c.Pattern != "",
		"target:allowed_values": c.AllowedValues != nil,
		"target:min":            c.Min != nil,
		"target:max":            c.Max != nil,
		"target:scale":          c.Scale != nil,
		"target:rounding":       c.Rounding != "",
		"target:decimal_sep":    c.DecimalSep != "",
		"target:thousands_sep":  c.ThousandsSep != "",
		"target:true_value":     c.TrueValue != nil,
		"target:false_value":    c.FalseValue != nil,
		"target:format":         c.Format != "",
		"target:timezone":       c.Timezone != "",
		"formats":               r.Formats != nil,
		"decimal_sep":           r.DecimalSep != "",
		"thousands_sep":         r.ThousandsSep != "",
		"true_values":           r.TrueValues != nil,
		"false_values":          r.FalseValues != nil,
		"timezone":              r.Timezone != "",
	}
	for _, name := range slices.Sorted(maps.Keys(set)) {
		if set[name] && !o.used[name] {
			o.errorf(name, "not valid for type %s", c.Type)
		}
	}
}

func scalars(list []config.Scalar) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = string(s)
	}
	return out
}

// shorten keeps error messages readable when a cell holds a huge value.
func shorten(s string) string {
	const max = 60
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
