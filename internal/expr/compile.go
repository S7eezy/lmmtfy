package expr

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Scope tells the compiler what a formula can refer to.
type Scope interface {
	// Column resolves [name] to an index into Env.Columns and its type.
	Column(name string) (index int, t Type, ok bool)
	// Value is the type of `value`; ok is false when the formula has no
	// source value (a computed column).
	Value() (t Type, ok bool)
}

// Env is what a formula reads when it is evaluated.
type Env struct {
	Value      Value     // `value`
	Columns    []Value   // indexed as resolved by Scope.Column
	Aggregates []Value   // indexed by Aggregate.Slot
	Row        int       // 1-based data row number, for row()
	Start      time.Time // when the conversion started, for today() and now()
}

// Program is a compiled formula.
type Program struct {
	Source    string
	Type      Type  // result type; Empty when it is always empty
	Columns   []int // indexes of the columns it reads, in order of appearance
	UsesValue bool
	// Volatile formulas (today, now) give different results from run to run.
	Volatile bool
	// UsesRow is true when the formula calls row() outside aggregates.
	UsesRow bool
	// Aggregates are the aggregate calls, outside other aggregates. Columns
	// their arguments read are in their own programs, not in Columns.
	Aggregates []*Aggregate
	eval       evalFunc
}

// Eval evaluates the formula. Errors are cell errors: division by zero,
// invalid dates, and the like.
func (p *Program) Eval(env *Env) (Value, error) { return p.eval(env) }

type evalFunc func(*Env) (Value, error)

// Compile parses and type-checks a formula.
func Compile(src string, scope Scope) (*Program, error) {
	n, err := parse(src)
	if err != nil {
		return nil, err
	}
	c := &compiler{scope: scope, prog: &Program{Source: src}}
	e, err := c.compile(n)
	if err != nil {
		return nil, err
	}
	c.prog.Type = e.t
	c.prog.eval = e.fn
	return c.prog, nil
}

// Assignable reports whether a formula giving from can fill a column of
// type to, and why not.
func Assignable(from, to Type) error {
	switch {
	case from == to, from == Empty, to == String, from == Integer && to == Decimal:
		return nil
	case from == Decimal && to == Integer:
		return errors.New("the formula gives a decimal number but the column is an integer: use round(…), floor(…) or ceil(…)")
	}
	return fmt.Errorf("the formula gives %s but the column is %s", from.label(), to.label())
}

// Convert turns a value into type t, which must be assignable from its
// type.
func Convert(v Value, t Type) Value {
	switch {
	case v.T == Empty || v.T == t:
		return v
	case t == String:
		return Text(v.Text())
	case t == Decimal && v.T == Integer:
		v.T = Decimal
	}
	return v
}

type compiler struct {
	scope       Scope
	prog        *Program
	inAggregate bool
}

// compiled is a compiled node: its evaluation, its type, and its value
// when it is constant (computed once, at compile time).
type compiled struct {
	fn    evalFunc
	t     Type
	konst *Value
	n     node
}

func constant(n node, v Value) compiled {
	return compiled{fn: func(*Env) (Value, error) { return v, nil }, t: v.T, konst: &v, n: n}
}

// fold evaluates a node made only of constants once. Errors become compile
// errors, at the node: `1 / 0` is reported when the mapping is loaded.
func fold(e compiled, args ...compiled) (compiled, error) {
	for _, a := range args {
		if a.konst == nil {
			return e, nil
		}
	}
	v, err := e.fn(&Env{})
	if err != nil {
		return e, errorAt(e.n.at(), "%v", err)
	}
	k := Convert(v, e.t)
	e.konst = &k
	e.fn = func(*Env) (Value, error) { return k, nil }
	return e, nil
}

// name describes a node in messages.
func name(n node) string {
	switch n := n.(type) {
	case *columnRef:
		return "[" + n.name + "]"
	case *valueRef:
		return "value"
	case *numberLit:
		return n.text
	case *stringLit:
		return fmt.Sprintf("%q", n.text)
	case *boolLit:
		if n.value {
			return "true"
		}
		return "false"
	case *call:
		return n.name + "(…)"
	}
	return "this part"
}

func (c *compiler) compile(n node) (compiled, error) {
	switch n := n.(type) {
	case *numberLit:
		d, err := decimal.NewFromString(n.text)
		if err != nil {
			return compiled{}, errorAt(n.pos, "%s is not a number", n.text)
		}
		t := Integer
		if strings.Contains(n.text, ".") {
			t = Decimal
		}
		return constant(n, Number(d, t)), nil

	case *stringLit:
		return constant(n, Text(n.text)), nil

	case *boolLit:
		return constant(n, Bool(n.value)), nil

	case *columnRef:
		idx, t, ok := c.scope.Column(n.name)
		if !ok {
			return compiled{}, errorAt(n.pos, "unknown column [%s]", n.name)
		}
		if !slices.Contains(c.prog.Columns, idx) {
			c.prog.Columns = append(c.prog.Columns, idx)
		}
		return compiled{fn: func(env *Env) (Value, error) { return env.Columns[idx], nil }, t: t, n: n}, nil

	case *valueRef:
		t, ok := c.scope.Value()
		if !ok {
			return compiled{}, errorAt(n.pos, "value is the source column's value, and this column has no source: refer to columns by name, like [price]")
		}
		c.prog.UsesValue = true
		return compiled{fn: func(env *Env) (Value, error) { return env.Value, nil }, t: t, n: n}, nil

	case *unary:
		x, err := c.compile(n.x)
		if err != nil {
			return compiled{}, err
		}
		var e compiled
		if n.op == "not" {
			if err := wantBool(x); err != nil {
				return compiled{}, err
			}
			e = compiled{fn: func(env *Env) (Value, error) {
				v, err := x.fn(env)
				return Bool(!truthy(v)), err
			}, t: Boolean, n: n}
		} else {
			if !x.t.numeric() && x.t != Empty {
				return compiled{}, errorAt(n.pos, "- needs a number, but %s is %s", name(x.n), x.t.label())
			}
			e = compiled{fn: func(env *Env) (Value, error) {
				v, err := x.fn(env)
				if err != nil || v.T == Empty {
					return v, err
				}
				return Number(v.n.Neg(), v.T), nil
			}, t: x.t, n: n}
		}
		return fold(e, x)

	case *binary:
		return c.binary(n)

	case *call:
		return c.call(n)
	}
	return compiled{}, errorAt(n.at(), "unexpected expression")
}

func truthy(v Value) bool { return v.T == Boolean && v.b }

func wantBool(x compiled) error {
	if x.t != Boolean && x.t != Empty {
		return errorAt(x.n.at(), "%s is %s, not a condition: compare it, like %s > 0", name(x.n), x.t.label(), name(x.n))
	}
	return nil
}

func unify(a, b Type) (Type, bool) {
	switch {
	case a == b:
		return a, true
	case a == Empty:
		return b, true
	case b == Empty:
		return a, true
	case a.numeric() && b.numeric():
		return Decimal, true
	}
	return Empty, false
}

func (c *compiler) binary(n *binary) (compiled, error) {
	l, err := c.compile(n.l)
	if err != nil {
		return compiled{}, err
	}
	r, err := c.compile(n.r)
	if err != nil {
		return compiled{}, err
	}

	var e compiled
	switch n.op {
	case "and", "or":
		if err := wantBool(l); err != nil {
			return compiled{}, err
		}
		if err := wantBool(r); err != nil {
			return compiled{}, err
		}
		and := n.op == "and"
		e = compiled{fn: func(env *Env) (Value, error) {
			a, err := l.fn(env)
			if err != nil {
				return a, err
			}
			if truthy(a) != and { // false and …, true or …
				return Bool(!and), nil
			}
			b, err := r.fn(env)
			return Bool(truthy(b)), err
		}, t: Boolean, n: n}

	case "+", "-", "*", "/", "%":
		for _, x := range []compiled{l, r} {
			switch {
			case x.t.numeric() || x.t == Empty:
			case n.op == "+" && x.t == String:
				return compiled{}, errorAt(n.pos, "+ adds numbers, but %s is text: use & to join text", name(x.n))
			case (n.op == "+" || n.op == "-") && (x.t == Date || x.t == Datetime):
				return compiled{}, errorAt(n.pos, "%s is %s: use add_days(…), add_months(…) or days_between(…)", name(x.n), x.t.label())
			default:
				return compiled{}, errorAt(n.pos, "%s needs numbers, but %s is %s", n.op, name(x.n), x.t.label())
			}
		}
		t, _ := unify(l.t, r.t)
		if t == Empty {
			t = Integer
		}
		if n.op == "/" {
			t = Decimal
		}
		op := n.op
		e = compiled{fn: func(env *Env) (Value, error) {
			a, err := l.fn(env)
			if err != nil {
				return a, err
			}
			b, err := r.fn(env)
			if err != nil || a.T == Empty || b.T == Empty {
				return Value{}, err
			}
			return arith(op, a, b, t)
		}, t: t, n: n}

	case "&":
		e = compiled{fn: func(env *Env) (Value, error) {
			a, err := l.fn(env)
			if err != nil {
				return a, err
			}
			b, err := r.fn(env)
			return Text(a.Text() + b.Text()), err
		}, t: String, n: n}

	default: // comparisons
		if err := comparable(n, l, r); err != nil {
			return compiled{}, err
		}
		op := n.op
		e = compiled{fn: func(env *Env) (Value, error) {
			a, err := l.fn(env)
			if err != nil {
				return a, err
			}
			b, err := r.fn(env)
			if err != nil || a.T == Empty || b.T == Empty {
				return Bool(false), err
			}
			c := compare(a, b)
			switch op {
			case "=":
				return Bool(c == 0), nil
			case "<>":
				return Bool(c != 0), nil
			case "<":
				return Bool(c < 0), nil
			case "<=":
				return Bool(c <= 0), nil
			case ">":
				return Bool(c > 0), nil
			}
			return Bool(c >= 0), nil
		}, t: Boolean, n: n}
	}
	return fold(e, l, r)
}

func comparable(n *binary, l, r compiled) error {
	if _, ok := unify(l.t, r.t); !ok {
		hint := ""
		switch {
		case l.t == String && r.t.numeric() || r.t == String && l.t.numeric():
			hint = ": write numbers without quotes, or convert with number(…) or text(…)"
		case l.t == String && r.t.temporal() || r.t == String && l.t.temporal():
			hint = ": write dates with date(\"2024-01-31\") or datetime(…)"
		}
		return errorAt(n.pos, "cannot compare %s (%s) with %s (%s)%s", name(l.n), l.t.label(), name(r.n), r.t.label(), hint)
	}
	if (l.t == Boolean || r.t == Boolean) && n.op != "=" && n.op != "<>" {
		return errorAt(n.pos, "true and false can only be compared with = or <>")
	}
	return nil
}

// divPrecision is the number of decimal places of a division result,
// before the target column rounds it.
const divPrecision = 34

func arith(op string, a, b Value, t Type) (Value, error) {
	var d decimal.Decimal
	switch op {
	case "+":
		d = a.n.Add(b.n)
	case "-":
		d = a.n.Sub(b.n)
	case "*":
		d = a.n.Mul(b.n)
	case "/":
		if b.n.IsZero() {
			return Value{}, errors.New("division by zero")
		}
		d = a.n.DivRound(b.n, divPrecision)
	case "%":
		if b.n.IsZero() {
			return Value{}, errors.New("division by zero")
		}
		d = a.n.Mod(b.n)
	}
	return Number(d, t), nil
}
