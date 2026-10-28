package expr

import (
	"testing"

	"github.com/shopspring/decimal"
)

func mustDec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// FuzzCompile checks that no formula makes the compiler or the evaluator
// panic.
func FuzzCompile(f *testing.F) {
	for _, s := range []string{
		"value / 1000", `if([price] > 10, "big", "small")`, `[name] & " " & [code]`,
		`ifs([qty] > 5, "many", "few")`, `from_epoch(value, "ms")`, `"a""b"`, "[a]]b]", "not not true",
		"round([price] / 7, 2)", `add_months(date("2024-01-31"), 1)`, "((1))", "-+-1",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		p, err := Compile(src, testScope{})
		if err != nil {
			return
		}
		p.Eval(testEnv(t))
	})
}
