package expr

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

// function is a function formulas can call. Arguments are evaluated before
// the call, except for the lazy functions handled by compiler.lazy.
type function struct {
	min, max int // number of arguments; max < 0: any number
	// check validates the argument types and gives the result type.
	check func(args []compiled) (Type, error)
	eval  func(args []Value, env *Env) (Value, error)
	// impure functions depend on the row or the run: never computed at
	// compile time.
	impure bool
}

// Functions lists the names of the functions formulas can call, sorted, for
// editors to offer.
func Functions() []string {
	names := []string{"if", "ifs", "coalesce"}
	for name := range aggregates {
		names = append(names, name)
	}
	for name := range functions {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

var functions map[string]function

func init() {
	functions = map[string]function{
		"is_empty": {min: 1, max: 1,
			check: func([]compiled) (Type, error) { return Boolean, nil },
			eval:  func(a []Value, _ *Env) (Value, error) { return Bool(a[0].T == Empty), nil }},

		// Numbers.
		"round": {min: 1, max: 2, check: func(a []compiled) (Type, error) {
			if err := want(a[0], "a number", Type.numeric); err != nil {
				return 0, err
			}
			if len(a) == 1 {
				return Integer, nil
			}
			return Decimal, want(a[1], "a whole number", isInteger)
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if anyEmpty(a) {
				return Value{}, nil
			}
			if len(a) == 1 {
				return Number(a[0].n.Round(0), Integer), nil
			}
			places, err := smallInt(a[1], "number of decimals")
			if err != nil {
				return Value{}, err
			}
			return Number(a[0].n.Round(int32(places)), Decimal), nil
		}},
		"floor": numeric1(Integer, decimal.Decimal.Floor),
		"ceil":  numeric1(Integer, decimal.Decimal.Ceil),
		"abs":   numeric1(Empty, decimal.Decimal.Abs),
		"min":   extreme(-1),
		"max":   extreme(1),
		"number": {min: 1, max: 1, check: textArgs(Decimal), eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty {
				return Value{}, nil
			}
			s := strings.TrimSpace(a[0].s)
			d, err := decimal.NewFromString(s)
			if err != nil || strings.ContainsAny(s, " ,_") {
				return Value{}, fmt.Errorf("%q is not a number (use . as decimal separator)", shorten(a[0].s))
			}
			return Number(d, Decimal), nil
		}},

		// Text.
		"upper": text1(strings.ToUpper),
		"lower": text1(strings.ToLower),
		"trim":  text1(strings.TrimSpace),
		"text": {min: 1, max: 1,
			check: func([]compiled) (Type, error) { return String, nil },
			eval:  func(a []Value, _ *Env) (Value, error) { return Text(a[0].Text()), nil }},
		"len": {min: 1, max: 1, check: textArgs(Integer), eval: func(a []Value, _ *Env) (Value, error) {
			return Int(int64(utf8.RuneCountInString(a[0].s))), nil
		}},
		"left":  cut(func(r []rune, n int) []rune { return r[:min(n, len(r))] }),
		"right": cut(func(r []rune, n int) []rune { return r[len(r)-min(n, len(r)):] }),
		"mid": {min: 3, max: 3, check: func(a []compiled) (Type, error) {
			return String, errors.Join(want(a[0], "text", isText), want(a[1], "a whole number", isInteger), want(a[2], "a whole number", isInteger))
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if anyEmpty(a) {
				return Value{}, nil
			}
			start, err := smallInt(a[1], "start")
			if err != nil {
				return Value{}, err
			}
			n, err := smallInt(a[2], "length")
			if err != nil {
				return Value{}, err
			}
			if start < 1 || n < 0 {
				return Value{}, errors.New("mid: start must be 1 or more and length 0 or more")
			}
			r := []rune(a[0].s)
			if start > len(r) {
				return Value{}, nil
			}
			return Text(string(r[start-1 : min(start-1+n, len(r))])), nil
		}},
		"replace": {min: 3, max: 3, check: textArgs(String), eval: func(a []Value, _ *Env) (Value, error) {
			if a[1].s == "" {
				return a[0], nil
			}
			return Text(strings.ReplaceAll(a[0].s, a[1].s, a[2].s)), nil
		}},
		"pad_left": {min: 3, max: 3, check: func(a []compiled) (Type, error) {
			return String, errors.Join(want(a[0], "text", isText), want(a[1], "a whole number", isInteger), want(a[2], "text", isText))
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty || a[1].T == Empty {
				return Value{}, nil
			}
			n, err := smallInt(a[1], "length")
			if err != nil {
				return Value{}, err
			}
			if utf8.RuneCountInString(a[2].s) != 1 {
				return Value{}, fmt.Errorf("pad_left: the padding must be a single character, got %q", a[2].s)
			}
			if missing := n - utf8.RuneCountInString(a[0].s); missing > 0 {
				return Text(strings.Repeat(a[2].s, missing) + a[0].s), nil
			}
			return a[0], nil
		}},
		"split_part": {min: 3, max: 3, check: func(a []compiled) (Type, error) {
			return String, errors.Join(want(a[0], "text", isText), want(a[1], "text", isText), want(a[2], "a whole number", isInteger))
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty || a[2].T == Empty {
				return Value{}, nil
			}
			if a[1].s == "" {
				return Value{}, errors.New("split_part: the separator is empty")
			}
			n, err := smallInt(a[2], "part number")
			if err != nil {
				return Value{}, err
			}
			if n < 1 {
				return Value{}, errors.New("split_part: the part number must be 1 or more")
			}
			parts := strings.Split(a[0].s, a[1].s)
			if n > len(parts) {
				return Value{}, nil
			}
			return Text(parts[n-1]), nil
		}},

		// Dates.
		"add_days": shift(func(t time.Time, n int) time.Time { return t.AddDate(0, 0, n) }),
		"add_months": shift(func(t time.Time, n int) time.Time {
			y, m, d := t.Date()
			first := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
			last := first.AddDate(0, 1, -1).Day()
			return first.AddDate(0, 0, min(d, last)-1) // Jan 31 + 1 month = Feb 28 or 29
		}),
		"days_between": {min: 2, max: 2, check: func(a []compiled) (Type, error) {
			if err := errors.Join(want(a[0], "a date or datetime", isDay), want(a[1], "a date or datetime", isDay)); err != nil {
				return 0, err
			}
			if _, ok := unify(a[0].t, a[1].t); !ok {
				return 0, errorAt(a[1].n.at(), "days_between needs two dates or two datetimes")
			}
			return Integer, nil
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if anyEmpty(a) {
				return Value{}, nil
			}
			return Int((a[1].t.Unix() - a[0].t.Unix()) / 86400), nil
		}},
		"year":   part(isDay, "a date or datetime", func(t time.Time) int { return t.Year() }),
		"month":  part(isDay, "a date or datetime", func(t time.Time) int { return int(t.Month()) }),
		"day":    part(isDay, "a date or datetime", func(t time.Time) int { return t.Day() }),
		"hour":   part(isClock, "a datetime or time", func(t time.Time) int { return t.Hour() }),
		"minute": part(isClock, "a datetime or time", func(t time.Time) int { return t.Minute() }),
		"from_epoch": {min: 2, max: 2, check: func(a []compiled) (Type, error) {
			return Datetime, errors.Join(want(a[0], "a number", Type.numeric), epochUnit(a[1]))
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty {
				return Value{}, nil
			}
			f, err := unitFactor(a[1].s)
			if err != nil {
				return Value{}, err
			}
			ns := a[0].n.Mul(decimal.NewFromInt(f)).Truncate(0).BigInt()
			if !ns.IsInt64() {
				return Value{}, fmt.Errorf("%s %s is too far from 1970", a[0].Text(), a[1].s)
			}
			return Moment(time.Unix(0, ns.Int64()).UTC(), Datetime), nil
		}},
		"to_epoch": {min: 2, max: 2, check: func(a []compiled) (Type, error) {
			return Integer, errors.Join(want(a[0], "a date or datetime", isDay), epochUnit(a[1]))
		}, eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty {
				return Value{}, nil
			}
			t := a[0].t
			switch a[1].s {
			case "s":
				return Int(t.Unix()), nil
			case "ms":
				return Int(t.UnixMilli()), nil
			case "us":
				return Int(t.UnixMicro()), nil
			}
			_, err := unitFactor(a[1].s)
			return Value{}, err
		}},
		"date": {min: 1, max: 1, check: textArgs(Date), eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty {
				return Value{}, nil
			}
			t, err := time.Parse(dateLayout, a[0].s)
			if err != nil {
				return Value{}, fmt.Errorf("%q is not a date: write it as YYYY-MM-DD", shorten(a[0].s))
			}
			return Moment(t, Date), nil
		}},
		"datetime": {min: 1, max: 1, check: textArgs(Datetime), eval: func(a []Value, _ *Env) (Value, error) {
			if a[0].T == Empty {
				return Value{}, nil
			}
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, a[0].s); err == nil {
					return Moment(t, Datetime), nil
				}
			}
			return Value{}, fmt.Errorf("%q is not a datetime: write it as YYYY-MM-DDTHH:mm:ss, with an optional offset (Z, +02:00)", shorten(a[0].s))
		}},
		"today": {impure: true,
			check: func([]compiled) (Type, error) { return Date, nil },
			eval: func(_ []Value, env *Env) (Value, error) {
				y, m, d := env.Start.Date()
				return Moment(time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Date), nil
			}},
		"now": {impure: true,
			check: func([]compiled) (Type, error) { return Datetime, nil },
			eval:  func(_ []Value, env *Env) (Value, error) { return Moment(env.Start, Datetime), nil }},

		// Row.
		"row": {impure: true,
			check: func([]compiled) (Type, error) { return Integer, nil },
			eval:  func(_ []Value, env *Env) (Value, error) { return Int(int64(env.Row)), nil }},
	}
}

func (c *compiler) call(n *call) (compiled, error) {
	switch n.name {
	case "if", "ifs", "coalesce":
		return c.lazy(n)
	}
	if IsAggregate(n.name) {
		return c.aggregate(n)
	}
	f, ok := functions[n.name]
	if !ok {
		return compiled{}, errorAt(n.pos, "unknown function %s", n.name)
	}
	if err := arity(n, f.min, f.max); err != nil {
		return compiled{}, err
	}
	args := make([]compiled, len(n.args))
	for i, a := range n.args {
		var err error
		if args[i], err = c.compile(a); err != nil {
			return compiled{}, err
		}
	}
	t, err := f.check(args)
	if err != nil {
		return compiled{}, err
	}
	if t == Empty { // same type as the first argument
		t = args[0].t
	}
	switch n.name {
	case "today", "now":
		c.prog.Volatile = true
	case "row":
		c.prog.UsesRow = true
	}

	eval := f.eval
	e := compiled{fn: func(env *Env) (Value, error) {
		vals := make([]Value, len(args))
		for i, a := range args {
			v, err := a.fn(env)
			if err != nil {
				return v, err
			}
			vals[i] = v
		}
		return eval(vals, env)
	}, t: t, n: n}
	if f.impure {
		return e, nil
	}
	return fold(e, args...)
}

// lazy compiles the functions evaluating only the arguments they need, so
// if([qty] = 0, 0, [total] / [qty]) never divides by zero.
func (c *compiler) lazy(n *call) (compiled, error) {
	min, max := 2, -1
	switch n.name {
	case "if":
		min, max = 2, 3
	case "ifs":
		min = 3
	}
	if err := arity(n, min, max); err != nil {
		return compiled{}, err
	}
	args := make([]compiled, len(n.args))
	for i, a := range n.args {
		var err error
		if args[i], err = c.compile(a); err != nil {
			return compiled{}, err
		}
	}

	// Conditions and results alternate; coalesce has only results.
	var conds, results []compiled
	switch n.name {
	case "coalesce":
		results = args
	case "if":
		conds, results = args[:1], args[1:]
	case "ifs":
		if len(args)%2 == 0 {
			return compiled{}, errorAt(n.pos, "ifs takes pairs of condition and result, then the result when no condition matches")
		}
		for i := 0; i+1 < len(args); i += 2 {
			conds = append(conds, args[i])
			results = append(results, args[i+1])
		}
		results = append(results, args[len(args)-1])
	}
	for _, cond := range conds {
		if err := wantBool(cond); err != nil {
			return compiled{}, err
		}
	}
	t := Empty
	for _, r := range results {
		u, ok := unify(t, r.t)
		if !ok {
			return compiled{}, errorAt(r.n.at(), "%s gives %s, but an earlier result is %s: all results must have the same type", name(r.n), r.t.label(), t.label())
		}
		t = u
	}

	var fn evalFunc
	if n.name == "coalesce" {
		fn = func(env *Env) (Value, error) {
			for _, r := range results {
				v, err := r.fn(env)
				if err != nil || v.T != Empty {
					return Convert(v, t), err
				}
			}
			return Value{}, nil
		}
	} else {
		fn = func(env *Env) (Value, error) {
			for i, cond := range conds {
				v, err := cond.fn(env)
				if err != nil {
					return v, err
				}
				if truthy(v) {
					r, err := results[i].fn(env)
					return Convert(r, t), err
				}
			}
			if len(results) > len(conds) {
				r, err := results[len(results)-1].fn(env)
				return Convert(r, t), err
			}
			return Value{}, nil
		}
	}
	return fold(compiled{fn: fn, t: t, n: n}, args...)
}

func arity(n *call, min, max int) error {
	got := len(n.args)
	switch {
	case max == min && got != min:
		return errorAt(n.pos, "%s takes %s, got %d", n.name, plural(min, "argument"), got)
	case got < min:
		return errorAt(n.pos, "%s takes at least %s, got %d", n.name, plural(min, "argument"), got)
	case max >= 0 && got > max:
		return errorAt(n.pos, "%s takes at most %s, got %d", n.name, plural(max, "argument"), got)
	}
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func want(a compiled, what string, ok func(Type) bool) error {
	if a.t == Empty || ok(a.t) {
		return nil
	}
	hint := ""
	if what == "text" {
		hint = ": convert it with text(…)"
	}
	return errorAt(a.n.at(), "expected %s, but %s is %s%s", what, name(a.n), a.t.label(), hint)
}

func isText(t Type) bool    { return t == String }
func isInteger(t Type) bool { return t == Integer }
func isDay(t Type) bool     { return t == Date || t == Datetime }
func isClock(t Type) bool   { return t == Datetime || t == Time }

func anyEmpty(a []Value) bool {
	return slices.ContainsFunc(a, Value.IsEmpty)
}

// smallInt reads an integer argument that must fit an int.
func smallInt(v Value, what string) (int, error) {
	if !v.n.IsInteger() || v.n.Abs().GreaterThan(decimal.NewFromInt(1_000_000)) {
		return 0, fmt.Errorf("%s %s is out of range", what, v.Text())
	}
	return int(v.n.IntPart()), nil
}

// textArgs checks that every argument is text.
func textArgs(result Type) func([]compiled) (Type, error) {
	return func(a []compiled) (Type, error) {
		var errs []error
		for _, x := range a {
			errs = append(errs, want(x, "text", isText))
		}
		return result, errors.Join(errs...)
	}
}

func numeric1(result Type, f func(decimal.Decimal) decimal.Decimal) function {
	return function{min: 1, max: 1, check: func(a []compiled) (Type, error) {
		return result, want(a[0], "a number", Type.numeric)
	}, eval: func(a []Value, _ *Env) (Value, error) {
		if a[0].T == Empty {
			return Value{}, nil
		}
		t := result
		if t == Empty {
			t = a[0].T
		}
		return Number(f(a[0].n), t), nil
	}}
}

// extreme is min (-1) or max (1) of its arguments, ignoring empty ones.
func extreme(sign int) function {
	return function{min: 2, max: -1, check: func(a []compiled) (Type, error) {
		t := Empty
		for _, x := range a {
			if x.t == Boolean {
				return 0, errorAt(x.n.at(), "true and false have no order")
			}
			u, ok := unify(t, x.t)
			if !ok {
				return 0, errorAt(x.n.at(), "%s is %s, but an earlier argument is %s", name(x.n), x.t.label(), t.label())
			}
			t = u
		}
		return t, nil
	}, eval: func(a []Value, _ *Env) (Value, error) {
		var best Value
		for _, v := range a {
			if v.T != Empty && (best.T == Empty || compare(v, best)*sign > 0) {
				best = v
			}
		}
		return best, nil
	}}
}

func text1(f func(string) string) function {
	return function{min: 1, max: 1, check: textArgs(String), eval: func(a []Value, _ *Env) (Value, error) {
		return Text(f(a[0].s)), nil
	}}
}

func cut(f func(r []rune, n int) []rune) function {
	return function{min: 2, max: 2, check: func(a []compiled) (Type, error) {
		return String, errors.Join(want(a[0], "text", isText), want(a[1], "a whole number", isInteger))
	}, eval: func(a []Value, _ *Env) (Value, error) {
		if anyEmpty(a) {
			return Value{}, nil
		}
		n, err := smallInt(a[1], "length")
		if err != nil {
			return Value{}, err
		}
		if n < 0 {
			return Value{}, errors.New("the length must be 0 or more")
		}
		return Text(string(f([]rune(a[0].s), n))), nil
	}}
}

func shift(f func(time.Time, int) time.Time) function {
	return function{min: 2, max: 2, check: func(a []compiled) (Type, error) {
		return Empty, errors.Join(want(a[0], "a date or datetime", isDay), want(a[1], "a whole number", isInteger))
	}, eval: func(a []Value, _ *Env) (Value, error) {
		if anyEmpty(a) {
			return Value{}, nil
		}
		n, err := smallInt(a[1], "count")
		if err != nil {
			return Value{}, err
		}
		return Moment(f(a[0].t, n), a[0].T), nil
	}}
}

func part(ok func(Type) bool, what string, f func(time.Time) int) function {
	return function{min: 1, max: 1, check: func(a []compiled) (Type, error) {
		return Integer, want(a[0], what, ok)
	}, eval: func(a []Value, _ *Env) (Value, error) {
		if a[0].T == Empty {
			return Value{}, nil
		}
		return Int(int64(f(a[0].t))), nil
	}}
}

func epochUnit(a compiled) error {
	if err := want(a, "text", isText); err != nil {
		return err
	}
	if a.konst != nil {
		if _, err := unitFactor(a.konst.s); err != nil {
			return errorAt(a.n.at(), "%v", err)
		}
	}
	return nil
}

// unitFactor is the number of nanoseconds in an epoch unit.
func unitFactor(unit string) (int64, error) {
	switch unit {
	case "s":
		return 1e9, nil
	case "ms":
		return 1e6, nil
	case "us":
		return 1e3, nil
	}
	return 0, fmt.Errorf("the unit must be \"s\", \"ms\" or \"us\", got %q", unit)
}

func shorten(s string) string {
	const max = 60
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
