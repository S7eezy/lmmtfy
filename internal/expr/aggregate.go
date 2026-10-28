package expr

import (
	"github.com/shopspring/decimal"
)

// Aggregate is an aggregate call in a formula, such as lowest([price]).
// Its argument is evaluated on every row by the engine, which folds the
// values with an Accumulator and puts the result in Env.Aggregates[Slot]
// before the formula runs.
type Aggregate struct {
	Kind string   // first, last, lowest, highest, total, mean, count
	Arg  *Program // nil for count()
	Type Type     // result type
	Slot int      // index in Env.Aggregates, set by the engine
}

var aggregates = map[string]bool{"first": true, "last": true, "lowest": true, "highest": true, "total": true, "mean": true, "count": true}

// Columns lists the columns the argument reads; none for count().
func (a *Aggregate) Columns() []int {
	if a.Arg == nil {
		return nil
	}
	return a.Arg.Columns
}

// IsAggregate reports whether a function name is an aggregate.
func IsAggregate(name string) bool { return aggregates[name] }

func (c *compiler) aggregate(n *call) (compiled, error) {
	if c.inAggregate {
		return compiled{}, errorAt(n.pos, "%s is inside another aggregate: aggregates cannot be nested", n.name)
	}
	min, max := 1, 1
	if n.name == "count" {
		min = 0
	}
	if err := arity(n, min, max); err != nil {
		return compiled{}, err
	}
	agg := &Aggregate{Kind: n.name, Type: Integer}
	if len(n.args) == 1 {
		sub := &compiler{scope: aggregateScope{c.scope}, prog: &Program{}, inAggregate: true}
		e, err := sub.compile(n.args[0])
		if err != nil {
			return compiled{}, err
		}
		sub.prog.Type, sub.prog.eval = e.t, e.fn
		agg.Arg = sub.prog
		switch n.name {
		case "first", "last":
			agg.Type = e.t
		case "lowest", "highest":
			if e.t == Boolean {
				return compiled{}, errorAt(e.n.at(), "true and false have no order")
			}
			agg.Type = e.t
		case "total":
			if err := want(e, "a number", Type.numeric); err != nil {
				return compiled{}, err
			}
			agg.Type = e.t
		case "mean":
			if err := want(e, "a number", Type.numeric); err != nil {
				return compiled{}, err
			}
			agg.Type = Decimal
		}
		if agg.Type == Empty {
			agg.Type = String
		}
	}
	c.prog.Aggregates = append(c.prog.Aggregates, agg)
	return compiled{fn: func(env *Env) (Value, error) {
		if agg.Slot >= len(env.Aggregates) {
			return Value{}, &Error{Msg: "aggregates are not computed"} // an engine bug
		}
		return env.Aggregates[agg.Slot], nil
	}, t: agg.Type, n: n}, nil
}

// aggregateScope is the scope of an aggregate's argument: columns, not the
// value of the column being computed.
type aggregateScope struct{ Scope }

func (aggregateScope) Value() (Type, bool) { return Empty, false }

// Accumulator folds the values of an aggregate's argument, row by row.
// Empty values are left out.
type Accumulator struct {
	agg    *Aggregate
	value  Value // first, last, lowest, highest
	sum    decimal.Decimal
	values int64 // non-empty values added
	rows   int64 // rows added, for count()
}

// NewAccumulator starts folding for an aggregate.
func NewAccumulator(a *Aggregate) *Accumulator { return &Accumulator{agg: a} }

// Add folds the value of the argument in one row (anything for count()).
func (a *Accumulator) Add(v Value) {
	a.rows++
	if v.T == Empty {
		return
	}
	a.values++
	switch a.agg.Kind {
	case "first":
		if a.values == 1 {
			a.value = v
		}
	case "last":
		a.value = v
	case "lowest":
		if a.values == 1 || compare(v, a.value) < 0 {
			a.value = v
		}
	case "highest":
		if a.values == 1 || compare(v, a.value) > 0 {
			a.value = v
		}
	case "total", "mean":
		a.sum = a.sum.Add(v.n)
	}
}

// Done reports whether more rows cannot change the result: a first value
// is found.
func (a *Accumulator) Done() bool { return a.agg.Kind == "first" && a.values > 0 }

// Result is the aggregate's value: empty without values, except count.
func (a *Accumulator) Result() Value {
	switch a.agg.Kind {
	case "count":
		if a.agg.Arg == nil {
			return Int(a.rows)
		}
		return Int(a.values)
	case "total":
		if a.values == 0 {
			return Value{}
		}
		return Number(a.sum, a.agg.Type)
	case "mean":
		if a.values == 0 {
			return Value{}
		}
		return Number(a.sum.DivRound(decimal.NewFromInt(a.values), divPrecision), Decimal)
	}
	return Convert(a.value, a.agg.Type)
}
