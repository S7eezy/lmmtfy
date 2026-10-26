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

	out, ok := c.fast(s)
	if !ok {
		var err error
		if out, err = c.exact(s, raw); err != nil {
			return "", err
		}
	}
	intPart, frac, hasFrac := strings.Cut(out, ".")
	out = group(intPart, c.outSep)
	if hasFrac {
		out += c.outDec + frac
	}
	return out, nil
}

// pow10[i] is 10^i, up to the largest power of ten fitting in an int64.
var pow10 = func() (p [19]int64) {
	p[0] = 1
	for i := 1; i < len(p); i++ {
		p[i] = p[i-1] * 10
	}
	return p
}()

// fast rounds and formats s (a valid decimal, "." separator) with int64
// arithmetic, which covers almost every real value: up to 18 significant
// digits. It returns false when s does not fit, uses scientific notation or
// min/max must be checked; the caller then uses exact. Output is always in
// plain notation (5.4e-07 → 0.00000054). Both give the same result: see the
// differential tests.
func (c *decConv) fast(s string) (string, bool) {
	if c.min != nil || c.max != nil || strings.ContainsAny(s, "eE") {
		return "", false
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	intPart, frac, _ := strings.Cut(s, ".")
	intPart = strings.TrimLeft(intPart, "0")
	digits := len(intPart) + len(frac)
	if digits > 18 {
		return "", false
	}

	// m is the value without its decimal point: 12.345 → 12345, scale 3.
	var m int64
	for i := 0; i < len(intPart); i++ {
		m = m*10 + int64(intPart[i]-'0')
	}
	for i := 0; i < len(frac); i++ {
		m = m*10 + int64(frac[i]-'0')
	}
	scale := len(frac)

	if c.hasScale {
		k := int(c.scale)
		switch {
		case scale < k:
			if digits+k-scale > 18 {
				return "", false
			}
			m *= pow10[k-scale]
		case scale > k:
			// Rounding works on the magnitude; the sign is put back below,
			// which gives half away from zero and symmetric half-even.
			p := pow10[scale-k]
			q, rem := m/p, m%p
			switch c.rounding {
			case "half_up":
				if rem*2 >= p {
					q++
				}
			case "half_even":
				if rem*2 > p || rem*2 == p && q%2 == 1 {
					q++
				}
			}
			m = q
		}
		scale = k
	} else {
		// Without a target scale the value is written as is, minus
		// trailing zeros: 1.50 → 1.5, 10.0 → 10.
		for scale > 0 && m%10 == 0 {
			m /= 10
			scale--
		}
	}

	b := make([]byte, 0, 24)
	if neg && m != 0 {
		b = append(b, '-')
	}
	str := strconv.FormatInt(m, 10)
	if scale == 0 {
		return string(append(b, str...)), true
	}
	if len(str) <= scale { // 0.05 → "5" needs leading zeros
		b = append(b, "0."...)
		for range scale - len(str) {
			b = append(b, '0')
		}
		return string(append(b, str...)), true
	}
	b = append(b, str[:len(str)-scale]...)
	b = append(b, '.')
	return string(append(b, str[len(str)-scale:]...)), true
}

// exact rounds, checks and formats s with arbitrary precision.
func (c *decConv) exact(s, raw string) (string, error) {
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
	if c.hasScale {
		return d.StringFixed(c.scale), nil
	}
	return d.String(), nil
}

// maxExponent bounds scientific notation to the range of a double
// (1.8e308): every real export fits, and "1e999999999" cannot make the
// output a billion digits long.
const maxExponent = 308

// isDecimal reports whether s is [+-]digits[.digits][e[+-]digits], with at
// least one mantissa digit. Scientific notation (5.4e-07) is what Python,
// Excel and lab tools write for very small or large numbers. No spaces:
// inputs are explicit about their format.
func isDecimal(s string) bool {
	mantissa, exp, hasExp := strings.Cut(strings.ToLower(s), "e")
	if hasExp && !validExponent(exp) {
		return false
	}
	if mantissa != "" && (mantissa[0] == '+' || mantissa[0] == '-') {
		mantissa = mantissa[1:]
	}
	digits, dot := 0, false
	for i := 0; i < len(mantissa); i++ {
		switch c := mantissa[i]; {
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

func validExponent(exp string) bool {
	if exp != "" && (exp[0] == '+' || exp[0] == '-') {
		exp = exp[1:]
	}
	n, err := strconv.Atoi(exp)
	return err == nil && exp != "" && n <= maxExponent && !strings.ContainsAny(exp, "+-")
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
