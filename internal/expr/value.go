package expr

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Type is the type of a value. The names are the target column types.
type Type uint8

const (
	// Empty is the type of the "" literal: it fits wherever a value is
	// expected. At run time, any value can be empty.
	Empty Type = iota
	String
	Integer
	Decimal
	Boolean
	Date
	Datetime
	Time
)

var typeNames = [...]string{"empty", "string", "integer", "decimal", "boolean", "date", "datetime", "time"}

func (t Type) String() string { return typeNames[t] }

// TypeOf returns the type named like a target column type ("decimal").
func TypeOf(name string) (Type, bool) {
	for i, n := range typeNames {
		if n == name && i != int(Empty) {
			return Type(i), true
		}
	}
	return Empty, false
}

// label names a type in messages, the way people say it.
func (t Type) label() string {
	switch t {
	case String:
		return "text"
	case Integer, Decimal:
		return "a number"
	case Boolean:
		return "true or false"
	case Empty:
		return "empty"
	}
	return "a " + t.String()
}

func (t Type) numeric() bool  { return t == Integer || t == Decimal }
func (t Type) temporal() bool { return t == Date || t == Datetime || t == Time }

// Value is a typed value. The zero Value is empty; text "" is empty too, so
// an empty cell and an empty text are the same thing.
type Value struct {
	T Type
	s string
	n decimal.Decimal
	b bool
	t time.Time
}

// Text returns a text value, empty for "".
func Text(s string) Value {
	if s == "" {
		return Value{}
	}
	return Value{T: String, s: s}
}

// Number returns an integer or decimal value.
func Number(d decimal.Decimal, t Type) Value { return Value{T: t, n: d} }

// Int returns an integer value.
func Int(n int64) Value { return Value{T: Integer, n: decimal.NewFromInt(n)} }

// Bool returns a boolean value.
func Bool(b bool) Value { return Value{T: Boolean, b: b} }

// Moment returns a date, datetime or time value.
func Moment(t time.Time, typ Type) Value { return Value{T: typ, t: t} }

// IsTrue reports whether the value is the boolean true. Conditions use it:
// an empty condition is false.
func (v Value) IsTrue() bool { return v.T == Boolean && v.b }

// IsEmpty reports whether the value is empty.
func (v Value) IsEmpty() bool { return v.T == Empty }

// Canonical text forms, used to pass values between formulas and column
// converters: plain numbers with "." as decimal separator, true/false, and
// these date formats (lmmtfy tokens, see types.Layout).
const (
	DateFormat     = "YYYY-MM-DD"
	DatetimeFormat = "YYYY-MM-DDTHH:mm:ss.SSSSSSSSSZ"
	TimeFormat     = "HH:mm:ss.SSSSSSSSS"
)

// The same formats as Go layouts.
const (
	dateLayout     = "2006-01-02"
	datetimeLayout = "2006-01-02T15:04:05.000000000Z07:00"
	timeLayout     = "15:04:05.000000000"
)

// Parse reads a value of type t from its canonical text. "" is empty.
func Parse(t Type, s string) (Value, error) {
	if s == "" {
		return Value{}, nil
	}
	switch t {
	case String:
		return Text(s), nil
	case Integer, Decimal:
		d, err := decimal.NewFromString(s)
		if err != nil {
			return Value{}, fmt.Errorf("%q is not a number", s)
		}
		return Number(d, t), nil
	case Boolean:
		switch s {
		case "true":
			return Bool(true), nil
		case "false":
			return Bool(false), nil
		}
		return Value{}, fmt.Errorf("%q is not true or false", s)
	case Date, Datetime, Time:
		layout := map[Type]string{Date: dateLayout, Datetime: datetimeLayout, Time: timeLayout}[t]
		tm, err := time.Parse(layout, s)
		if err != nil {
			return Value{}, fmt.Errorf("%q is not a canonical %s", s, t)
		}
		return Moment(tm, t), nil
	}
	return Value{}, fmt.Errorf("cannot read a value of type %s", t)
}

// Canonical returns the canonical text of the value, "" when empty.
func (v Value) Canonical() string {
	switch v.T {
	case String:
		return v.s
	case Integer, Decimal:
		return v.n.String()
	case Boolean:
		if v.b {
			return "true"
		}
		return "false"
	case Date:
		return v.t.Format(dateLayout)
	case Datetime:
		return v.t.Format(datetimeLayout)
	case Time:
		return v.t.Format(timeLayout)
	}
	return ""
}

// Text returns the value as people read it: what text() gives and what &
// joins. Dates are ISO 8601, fractions of seconds only when there are some.
func (v Value) Text() string {
	switch v.T {
	case Date:
		return v.t.Format(dateLayout)
	case Datetime:
		return v.t.Format(time.RFC3339Nano)
	case Time:
		return strings.TrimSuffix(strings.TrimRight(v.t.Format(timeLayout), "0"), ".")
	}
	return v.Canonical()
}

// Rounded returns a decimal rounded to places, for display; other values
// as they are.
func (v Value) Rounded(places int32) Value {
	if v.T == Decimal {
		v.n = v.n.Round(places)
	}
	return v
}

// String is for debugging and tests: the text, typed.
func (v Value) String() string {
	if v.T == Empty {
		return "empty"
	}
	return v.T.String() + "(" + v.Text() + ")"
}

// Compare orders two values of the same type, empty values last.
func Compare(a, b Value) int {
	switch {
	case a.T == Empty && b.T == Empty:
		return 0
	case a.T == Empty:
		return 1
	case b.T == Empty:
		return -1
	}
	return compare(a, b)
}

// compare orders two non-empty values of compatible types.
func compare(a, b Value) int {
	switch {
	case a.T.numeric():
		return a.n.Cmp(b.n)
	case a.T == String:
		return strings.Compare(a.s, b.s)
	case a.T == Boolean:
		switch {
		case a.b == b.b:
			return 0
		case !a.b:
			return -1
		}
		return 1
	}
	return a.t.Compare(b.t)
}
