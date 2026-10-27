package types

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
)

func decimalConverters(t testing.TB) []*decConv {
	t.Helper()
	var convs []*decConv
	for _, rounding := range config.Roundings {
		for _, scale := range []*int{nil, n(0), n(1), n(2), n(4), n(9)} {
			conv, err := New(config.TargetColumn{Type: "decimal", Scale: scale, Rounding: rounding}, &config.Rule{})
			if err != nil {
				t.Fatal(err)
			}
			convs = append(convs, conv.(*decConv))
		}
	}
	return convs
}

// checkFastMatchesExact fails when the int64 fast path disagrees with the
// arbitrary-precision path on s.
func checkFastMatchesExact(t *testing.T, convs []*decConv, s string) {
	t.Helper()
	if !isDecimal(s) {
		return
	}
	for _, c := range convs {
		fast, ok := c.fast(s)
		if !ok {
			continue
		}
		exact, err := c.exact(s, s)
		if err != nil {
			t.Fatalf("exact(%q): %v", s, err)
		}
		if fast != exact {
			scale := "none"
			if c.hasScale {
				scale = strings.Repeat("#", int(c.scale))
			}
			t.Fatalf("%q, scale %s, %s: fast = %q, exact = %q", s, scale, c.rounding, fast, exact)
		}
	}
}

func TestDecimalFastPathMatchesExact(t *testing.T) {
	convs := decimalConverters(t)
	fixed := []string{
		"0", "-0", "+0", "0.0", "-0.000", ".5", "5.", "-.5", "007.50", "10.0", "100",
		"0.005", "-0.005", "0.015", "0.025", "-2.345", "2.355", "1.0000000005",
		"999999999999999999", "99999999999999999.9", "-999999999.999999999",
		"123456789012345678", "0.000000000000000001", "1234567890123456789",
		"5.419198916047157e-07", "1e5", "-1.5E+3",
	}
	for _, s := range fixed {
		checkFastMatchesExact(t, convs, s)
	}

	rng := rand.New(rand.NewPCG(1, 2))
	digits := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte('0' + rng.IntN(10))
		}
		return string(b)
	}
	for range 200000 {
		var s string
		switch rng.IntN(3) {
		case 0:
			s = "-"
		case 1:
			s = "+"
		}
		s += digits(rng.IntN(12))
		if rng.IntN(4) > 0 {
			s += "." + digits(rng.IntN(12))
		}
		// Bias towards exact halves, the tricky case for rounding.
		if rng.IntN(5) == 0 && strings.Contains(s, ".") {
			s += "5"
		}
		checkFastMatchesExact(t, convs, s)
	}
}

func FuzzDecimalFastPath(f *testing.F) {
	for _, s := range []string{"1.5", "-0.005", "123456789.123456789", ".5", "0"} {
		f.Add(s)
	}
	convs := decimalConverters(f)
	f.Fuzz(func(t *testing.T, s string) {
		checkFastMatchesExact(t, convs, s)
	})
}
