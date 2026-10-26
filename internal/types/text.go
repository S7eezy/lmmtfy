package types

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"lmmtfy/internal/config"
)

type stringConv struct {
	caseMode string
	maxLen   int
	truncate bool
	pattern  *regexp.Regexp
	allowed  map[string]bool
}

func newString(o *opts) Converter {
	o.use("target:max_length", "target:on_overflow", "target:case", "target:pattern", "target:allowed_values")
	c := &stringConv{caseMode: o.col.Case}

	if c.caseMode != "" && !slices.Contains(config.Cases, c.caseMode) {
		o.errorf("target:case", "%q is not one of: %s", c.caseMode, strings.Join(config.Cases, ", "))
	}
	if n := o.col.MaxLength; n != nil {
		if *n < 1 {
			o.errorf("target:max_length", "must be 1 or more")
		}
		c.maxLen = *n
	}
	switch o.col.OnOverflow {
	case "", "error":
	case "truncate":
		c.truncate = true
	default:
		o.errorf("target:on_overflow", "%q is not one of: %s", o.col.OnOverflow, strings.Join(config.Overflows, ", "))
	}
	if p := o.col.Pattern; p != "" {
		// The pattern must match the whole value, not just a part of it.
		re, err := regexp.Compile("^(?:" + p + ")$")
		if err != nil {
			o.errorf("target:pattern", "invalid regular expression: %v", err)
		}
		c.pattern = re
	}
	if o.col.AllowedValues != nil {
		c.allowed = map[string]bool{}
		for _, v := range o.col.AllowedValues {
			c.allowed[string(v)] = true
		}
	}
	return c
}

func (c *stringConv) Convert(raw string) (string, error) {
	v := raw
	switch c.caseMode {
	case "upper":
		v = strings.ToUpper(v)
	case "lower":
		v = strings.ToLower(v)
	}

	var warn error
	if c.maxLen > 0 && utf8.RuneCountInString(v) > c.maxLen {
		if !c.truncate {
			return "", fmt.Errorf("%q is longer than %d characters", shorten(v), c.maxLen)
		}
		v = string([]rune(v)[:c.maxLen])
		warn = &Warning{Msg: fmt.Sprintf("truncated to %d characters", c.maxLen)}
	}
	if c.pattern != nil && !c.pattern.MatchString(v) {
		return "", fmt.Errorf("%q does not match pattern %s", shorten(v), c.pattern)
	}
	if c.allowed != nil && !c.allowed[v] {
		return "", fmt.Errorf("%q is not an allowed value", shorten(v))
	}
	return v, warn
}

type boolConv struct {
	trueSet, falseSet map[string]bool
	outTrue, outFalse string
	expected          string
}

var (
	defaultTrue  = []string{"true", "t", "yes", "y", "1"}
	defaultFalse = []string{"false", "f", "no", "n", "0"}
)

func newBoolean(o *opts) Converter {
	o.use("target:true_value", "target:false_value", "true_values", "false_values")
	c := &boolConv{outTrue: "true", outFalse: "false"}
	if v := o.col.TrueValue; v != nil {
		c.outTrue = string(*v)
	}
	if v := o.col.FalseValue; v != nil {
		c.outFalse = string(*v)
	}
	if c.outTrue == c.outFalse {
		o.errorf("target:false_value", "must differ from true_value")
	}

	trueVals, falseVals := defaultTrue, defaultFalse
	switch {
	case o.fromOutput:
		trueVals, falseVals = []string{c.outTrue}, []string{c.outFalse}
	default:
		if o.rule.TrueValues != nil {
			trueVals = scalars(o.rule.TrueValues)
		}
		if o.rule.FalseValues != nil {
			falseVals = scalars(o.rule.FalseValues)
		}
	}

	// Matching is case-insensitive.
	c.trueSet, c.falseSet = map[string]bool{}, map[string]bool{}
	for _, v := range trueVals {
		c.trueSet[strings.ToLower(v)] = true
	}
	for _, v := range falseVals {
		v = strings.ToLower(v)
		if c.trueSet[v] {
			o.errorf("false_values", "%q is in both true_values and false_values", v)
		}
		c.falseSet[v] = true
	}
	c.expected = strings.Join(append(slices.Clone(trueVals), falseVals...), ", ")
	return c
}

func (c *boolConv) Convert(raw string) (string, error) {
	v := strings.ToLower(raw)
	switch {
	case c.trueSet[v]:
		return c.outTrue, nil
	case c.falseSet[v]:
		return c.outFalse, nil
	}
	return "", fmt.Errorf("%q is not a boolean (expected one of: %s)", shorten(raw), c.expected)
}
