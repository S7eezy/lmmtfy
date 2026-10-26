package types

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"lmmtfy/internal/config"
)

type intConv struct {
	inSep, outSep string
	min, max      *int64
}

func newInteger(o *opts) Converter {
	o.use("target:min", "target:max", "target:thousands_sep", "thousands_sep")
	c := &intConv{inSep: string(o.rule.ThousandsSep), outSep: string(o.col.ThousandsSep)}
	if o.fromOutput {
		c.inSep = c.outSep
	}
	checkSeparator(o, "thousands_sep", c.inSep)
	checkSeparator(o, "target:thousands_sep", c.outSep)

	parse := func(option string, s *config.Scalar) *int64 {
		if s == nil {
			return nil
		}
		n, err := strconv.ParseInt(string(*s), 10, 64)
		if err != nil {
			o.errorf(option, "%q is not an integer", *s)
			return nil
		}
		return &n
	}
	c.min = parse("target:min", o.col.Min)
	c.max = parse("target:max", o.col.Max)
	return c
}

func (c *intConv) Convert(raw string) (string, error) {
	s := raw
	if c.inSep != "" {
		s = strings.ReplaceAll(s, c.inSep, "")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return "", fmt.Errorf("%q is out of the 64-bit integer range", shorten(raw))
		}
		return "", fmt.Errorf("%q is not an integer", shorten(raw))
	}
	if c.min != nil && n < *c.min {
		return "", fmt.Errorf("%d is less than the minimum %d", n, *c.min)
	}
	if c.max != nil && n > *c.max {
		return "", fmt.Errorf("%d is more than the maximum %d", n, *c.max)
	}
	return group(strconv.FormatInt(n, 10), c.outSep), nil
}

type decConv struct {
	inDec, inSep   string
	outDec, outSep string
	scale          int32
	hasScale       bool
	rounding       string
	min, max       *decimal.Decimal
}

func newDecimal(o *opts) Converter {
	o.use("target:min", "target:max", "target:scale", "target:rounding",
		"target:decimal_sep", "target:thousands_sep", "decimal_sep", "thousands_sep")
	c := &decConv{
		outDec:   string(o.col.DecimalSep),
		outSep:   string(o.col.ThousandsSep),
		inDec:    string(o.rule.DecimalSep),
		inSep:    string(o.rule.ThousandsSep),
		rounding: o.col.Rounding,
	}
	if c.outDec == "" {
		c.outDec = "."
	}
	if c.inDec == "" {
		c.inDec = "."
	}
	if o.fromOutput {
		c.inDec, c.inSep = c.outDec, c.outSep
	}

	checkDecimalSep(o, "decimal_sep", c.inDec)
	checkDecimalSep(o, "target:decimal_sep", c.outDec)
	checkSeparator(o, "thousands_sep", c.inSep)
	checkSeparator(o, "target:thousands_sep", c.outSep)
	if c.inSep == c.inDec {
		o.errorf("thousands_sep", "must differ from decimal_sep")
	}
	if c.outSep == c.outDec {
		o.errorf("target:thousands_sep", "must differ from decimal_sep")
	}

	if s := o.col.Scale; s != nil {
		if *s < 0 || *s > 30 {
			o.errorf("target:scale", "must be between 0 and 30")
		}
		c.scale, c.hasScale = int32(*s), true
	}
	if c.rounding == "" {
		c.rounding = config.Roundings[0]
	}
	if !slices.Contains(config.Roundings, c.rounding) {
		o.errorf("target:rounding", "%q is not one of: %s", c.rounding, strings.Join(config.Roundings, ", "))
	}

	parse := func(option string, s *config.Scalar) *decimal.Decimal {
		if s == nil {
			return nil
		}
		d, err := decimal.NewFromString(string(*s))
		if err != nil || !isDecimal(string(*s)) {
			o.errorf(option, "%q is not a decimal number (use . as decimal separator)", *s)
			return nil
		}
		return &d
	}
	c.min = parse("target:min", o.col.Min)
	c.max = parse("target:max", o.col.Max)
	return c
}

func (c *decConv) Convert(raw string) (string, error) {
	s := raw
	if c.inSep != "" {
		s = strings.ReplaceAll(s, c.inSep, "")
	}
	if c.inDec != "." {
		if strings.Contains(s, ".") {
			return "", fmt.Errorf("%q is not a decimal number (decimal separator is %q)", shorten(raw), c.inDec)
		}
		s = strings.ReplaceAll(s, c.inDec, ".")
	}
	if !isDecimal(s) {
		return "", fmt.Errorf("%q is not a decimal number", shorten(raw))
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return "", fmt.Errorf("%q is not a decimal number", shorten(raw))
	}

	if c.hasScale {
		switch c.rounding {
		case "half_up":
			d = d.Round(c.scale) // half away from zero: 2.345 → 2.35, -2.345 → -2.35
		case "half_even":
			d = d.RoundBank(c.scale)
		case "truncate":
			d = d.Truncate(c.scale)
		}
	}
	if c.min != nil && d.LessThan(*c.min) {
		return "", fmt.Errorf("%s is less than the minimum %s", d, c.min)
	}
	if c.max != nil && d.GreaterThan(*c.max) {
		return "", fmt.Errorf("%s is more than the maximum %s", d, c.max)
	}

	out := d.String()
	if c.hasScale {
		out = d.StringFixed(c.scale)
	}
	intPart, frac, hasFrac := strings.Cut(out, ".")
	out = group(intPart, c.outSep)
	if hasFrac {
		out += c.outDec + frac
	}
	return out, nil
}

// isDecimal reports whether s is [+-]digits[.digits], with at least one digit.
// No exponent, no spaces: inputs are explicit about their format.
func isDecimal(s string) bool {
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	digits, dot := 0, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			digits++
		case c == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return digits > 0
}

// group inserts sep every three digits of an integer string ("-1234567" →
// "-1 234 567" with sep " ").
func group(digits, sep string) string {
	if sep == "" {
		return digits
	}
	sign := ""
	if digits != "" && (digits[0] == '-' || digits[0] == '+') {
		sign, digits = digits[:1], digits[1:]
	}
	if len(digits) <= 3 {
		return sign + digits
	}
	var b strings.Builder
	b.WriteString(sign)
	first := len(digits) % 3
	if first > 0 {
		b.WriteString(digits[:first])
	}
	for i := first; i < len(digits); i += 3 {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

func checkSeparator(o *opts, option, sep string) {
	if strings.ContainsAny(sep, "0123456789+-") {
		o.errorf(option, "%q cannot contain digits or signs", sep)
	}
}

func checkDecimalSep(o *opts, option, sep string) {
	if utf8.RuneCountInString(sep) != 1 {
		o.errorf(option, "must be a single character, got %q", sep)
	}
	checkSeparator(o, option, sep)
}
