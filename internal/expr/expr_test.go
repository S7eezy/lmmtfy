package expr

import (
	"strings"
	"testing"
	"time"
)

// testScope has a few columns of each type, and `value` as a decimal.
type testScope struct{ noValue bool }

var testColumns = []struct {
	name string
	t    Type
	text string // canonical value in testEnv
}{
	{"price", Decimal, "12.5"},
	{"qty", Integer, "3"},
	{"zero", Integer, "0"},
	{"name", String, "Ada Lovelace"},
	{"code", String, "a-b-c"},
	{"day", Date, "2024-01-31"},
	{"at", Datetime, "2024-03-10T08:30:00.000000000+02:00"},
	{"clock", Time, "14:05:09.000000000"},
	{"ok", Boolean, "true"},
	{"blank", Decimal, ""},
	{"blank text", String, ""},
}

func (s testScope) Column(name string) (int, Type, bool) {
	for i, c := range testColumns {
		if c.name == name {
			return i, c.t, true
		}
	}
	return 0, Empty, false
}

func (s testScope) Value() (Type, bool) { return Decimal, !s.noValue }

func testEnv(t *testing.T) *Env {
	env := &Env{Row: 7, Start: time.Date(2025, 6, 1, 23, 30, 0, 0, time.FixedZone("", 3600))}
	for _, c := range testColumns {
		v, err := Parse(c.t, c.text)
		if err != nil {
			t.Fatal(err)
		}
		env.Columns = append(env.Columns, v)
	}
	env.Value = Number(mustDec("1234567"), Decimal)
	return env
}

func TestEval(t *testing.T) {
	cases := []struct {
		src  string
		want string // Value.String()
		typ  Type
	}{
		// Arithmetic, exact.
		{"value / 1000", "decimal(1234.567)", Decimal},
		{"[price] * [qty]", "decimal(37.5)", Decimal},
		{"[qty] + 2 * 3", "integer(9)", Integer},
		{"([qty] + 2) * 3", "integer(15)", Integer},
		{"-[qty] % 2", "integer(-1)", Integer},
		{"0.1 + 0.2", "decimal(0.3)", Decimal},
		{"1 / 3 * 3", "decimal(0.9999999999999999999999999999999999)", Decimal},
		{"[blank] + 1", "empty", Decimal},
		{"10 - - 2", "integer(12)", Integer},

		// Text.
		{`[name] & " (" & [qty] & ")"`, "string(Ada Lovelace (3))", String},
		{`"say ""hi"""`, `string(say "hi")`, String},
		{`[blank text] & "x"`, "string(x)", String},
		{"upper(left([name], 3))", "string(ADA)", String},
		{"right([name], 8)", "string(Lovelace)", String},
		{"mid([name], 5, 4)", "string(Love)", String},
		{"mid([name], 50, 4)", "empty", String},
		{"len([name])", "integer(12)", Integer},
		{"len([blank text])", "integer(0)", Integer},
		{`replace([code], "-", "")`, "string(abc)", String},
		{`pad_left(text([qty]), 4, "0")`, "string(0003)", String},
		{`split_part([code], "-", 2)`, "string(b)", String},
		{`split_part([code], "-", 9)`, "empty", String},
		{`trim("  x ")`, "string(x)", String},
		{`number("-12.50") * 2`, "decimal(-25)", Decimal},
		{"text([day])", "string(2024-01-31)", String},
		{"text([ok])", "string(true)", String},

		// Numbers.
		{"round(2.5)", "integer(3)", Integer},
		{"round(-2.5)", "integer(-3)", Integer},
		{"round([price] / 7, 2)", "decimal(1.79)", Decimal},
		{"floor(-1.5)", "integer(-2)", Integer},
		{"ceil(1.1)", "integer(2)", Integer},
		{"abs(-[qty])", "integer(3)", Integer},
		{"max([qty], [price], 4)", "decimal(12.5)", Decimal},
		{"min([blank], 4, 9)", "integer(4)", Decimal},

		// Logic.
		{"[price] > 10", "boolean(true)", Boolean},
		{"[blank] > 10", "boolean(false)", Boolean},
		{"[blank] <> 10", "boolean(false)", Boolean},
		{"[name] = \"Ada Lovelace\" and not [ok]", "boolean(false)", Boolean},
		{"[qty] = 3 or 1 / [zero] > 0", "boolean(true)", Boolean},
		{`if([price] > 10, "big", "small")`, "string(big)", String},
		{`if([price] > 100, "big")`, "empty", String},
		{"if([zero] = 0, 0, [qty] / [zero])", "decimal(0)", Decimal},
		{`ifs([qty] > 5, "many", [qty] > 1, "some", "one")`, "string(some)", String},
		{"coalesce([blank], [qty])", "decimal(3)", Decimal},
		{"is_empty([blank])", "boolean(true)", Boolean},
		{`if(is_empty([blank]), "", "x")`, "empty", String},
		{"[day] < date(\"2024-02-01\")", "boolean(true)", Boolean},
		{"not is_empty([blank]) or [ok]", "boolean(true)", Boolean},

		// Dates.
		{"add_days([day], 1)", "date(2024-02-01)", Date},
		{"add_months([day], 1)", "date(2024-02-29)", Date},
		{"add_months([day], -2)", "date(2023-11-30)", Date},
		{`days_between(date("2024-01-01"), [day])`, "integer(30)", Integer},
		{"year([day]) * 100 + month([day])", "integer(202401)", Integer},
		{"hour([at])", "integer(8)", Integer},
		{"minute([clock])", "integer(5)", Integer},
		{`from_epoch(1700000000123, "ms")`, "datetime(2023-11-14T22:13:20.123Z)", Datetime},
		{`from_epoch(1.5, "s")`, "datetime(1970-01-01T00:00:01.5Z)", Datetime},
		{`to_epoch([at], "s")`, "integer(1710052200)", Integer},
		{`datetime("2024-01-31 10:00:00")`, "datetime(2024-01-31T10:00:00Z)", Datetime},
		{"today()", "date(2025-06-01)", Date},
		{"row() * 10", "integer(70)", Integer},

		// Names are case-insensitive, spaces inside brackets trimmed.
		{"UPPER([ code ])", "string(A-B-C)", String},
		{"IF(TRUE, 1, 2)", "integer(1)", Integer},
	}
	env := testEnv(t)
	for _, c := range cases {
		p, err := Compile(c.src, testScope{})
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if p.Type != c.typ {
			t.Errorf("%s: type %s, want %s", c.src, p.Type, c.typ)
		}
		v, err := p.Eval(env)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if v.String() != c.want {
			t.Errorf("%s = %s, want %s", c.src, v, c.want)
		}
	}
}

func TestCompileErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{"", "the formula is empty"},
		{"[price] +", "unexpected end of formula (character 10)"},
		{"[nope] * 2", "unknown column [nope] (character 1)"},
		{"price * 2", `unknown name "price": write columns in brackets`},
		{"frob(1)", "unknown function frob (character 1)"},
		{`"abc`, "text is not closed"},
		{"[abc", "column name is not closed"},
		{"[name] + 1", "+ adds numbers, but [name] is text: use & to join text (character 8)"},
		{"[day] + 1", "use add_days"},
		{"[name] > 5", "cannot compare [name] (text) with 5 (a number): write numbers without quotes"},
		{"[ok] < true", "true and false can only be compared with = or <>"},
		{"if([qty], 1, 2)", "[qty] is a number, not a condition"},
		{`if([ok], 1, "x")`, `"x" gives text, but an earlier result is a number`},
		{"ifs([ok], 1)", "ifs takes at least 3 arguments, got 2"},
		{"ifs([ok], 1, [ok], 2)", "ifs takes pairs"},
		{"round([name])", "expected a number, but [name] is text"},
		{"upper([qty])", "expected text, but [qty] is a number: convert it with text(…)"},
		{"left([name])", "left takes 2 arguments, got 1"},
		{"1 / 0", "division by zero (character 3)"},
		{`date("31/01/2024")`, `"31/01/2024" is not a date: write it as YYYY-MM-DD`},
		{`from_epoch(1, "h")`, `the unit must be "s", "ms" or "us", got "h"`},
		{"(1 + 2", "expected ) but found end of formula"},
		{"1 2", "unexpected number 2"},
		{"max([ok], 1)", "true and false have no order"},
		{"#", `unexpected '#'`},
	}
	for _, c := range cases {
		_, err := Compile(c.src, testScope{})
		if err == nil {
			t.Errorf("%s: no error, want %q", c.src, c.want)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q, want %q", c.src, err, c.want)
		}
	}

	if _, err := Compile("value * 2", testScope{noValue: true}); err == nil || !strings.Contains(err.Error(), "has no source") {
		t.Errorf("value without source: %v", err)
	}
}

func TestRuntimeErrors(t *testing.T) {
	env := testEnv(t)
	for _, c := range []struct{ src, want string }{
		{"[qty] / [zero]", "division by zero"},
		{`number([name])`, `"Ada Lovelace" is not a number`},
		{`date([code])`, `"a-b-c" is not a date`},
		{`split_part([code], "", 1)`, "separator is empty"},
	} {
		p, err := Compile(c.src, testScope{})
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		_, err = p.Eval(env)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.src, err, c.want)
		}
	}
}

func TestProgramFacts(t *testing.T) {
	p, err := Compile("[qty] * [price] + [qty] + value", testScope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Columns) != 2 || p.Columns[0] != 1 || p.Columns[1] != 0 || !p.UsesValue || p.Volatile {
		t.Errorf("columns %v, uses value %v, volatile %v", p.Columns, p.UsesValue, p.Volatile)
	}
	if p, _ := Compile("now()", testScope{}); !p.Volatile {
		t.Error("now() is not volatile")
	}
}

func TestCanonicalRoundTrip(t *testing.T) {
	for _, c := range testColumns {
		v, err := Parse(c.t, c.text)
		if err != nil {
			t.Fatal(err)
		}
		if got := v.Canonical(); got != c.text {
			t.Errorf("%s: canonical %q, want %q", c.name, got, c.text)
		}
	}
}

func TestAssignable(t *testing.T) {
	ok := [][2]Type{{Integer, Decimal}, {Decimal, String}, {Date, String}, {Empty, Date}, {Boolean, Boolean}}
	for _, p := range ok {
		if err := Assignable(p[0], p[1]); err != nil {
			t.Errorf("%s → %s: %v", p[0], p[1], err)
		}
	}
	if err := Assignable(Decimal, Integer); err == nil || !strings.Contains(err.Error(), "round(") {
		t.Errorf("decimal → integer: %v", err)
	}
	if err := Assignable(String, Integer); err == nil {
		t.Error("text → integer accepted")
	}
}

func TestDeepNesting(t *testing.T) {
	src := strings.Repeat("(", 10000) + "1" + strings.Repeat(")", 10000)
	if _, err := Compile(src, testScope{}); err == nil || !strings.Contains(err.Error(), "nested too deeply") {
		t.Errorf("deep nesting: %v", err)
	}
	if _, err := Compile(strings.Repeat("-", 10000)+"1", testScope{}); err == nil {
		t.Error("deep unary accepted")
	}
}

func TestDocsCoverFunctions(t *testing.T) {
	documented := map[string]bool{}
	for _, d := range Docs() {
		if documented[d.Name] {
			t.Errorf("%s documented twice", d.Name)
		}
		documented[d.Name] = true
	}
	for _, name := range Functions() {
		if !documented[name] {
			t.Errorf("%s is not documented", name)
		}
		delete(documented, name)
	}
	for name := range documented {
		t.Errorf("%s is documented but does not exist", name)
	}
}

func TestAggregates(t *testing.T) {
	p, err := Compile("[price] / total([price] * [qty]) + count()", testScope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Aggregates) != 2 || len(p.Columns) != 1 || p.Type != Decimal {
		t.Fatalf("aggregates %d, columns %v, type %s", len(p.Aggregates), p.Columns, p.Type)
	}
	total, count := p.Aggregates[0], p.Aggregates[1]
	if total.Kind != "total" || total.Type != Decimal || len(total.Arg.Columns) != 2 || count.Arg != nil {
		t.Fatalf("%+v %+v", total, count)
	}

	// Fold three rows: price 2, 3, empty with qty 10.
	acc := map[string]*Accumulator{}
	for _, kind := range []string{"first", "last", "lowest", "highest", "total", "mean", "count"} {
		q, err := Compile(kind+"([price])", testScope{})
		if err != nil {
			t.Fatal(err)
		}
		acc[kind] = NewAccumulator(q.Aggregates[0])
	}
	for _, v := range []string{"3", "2", "", "5"} {
		val, _ := Parse(Decimal, v)
		for _, a := range acc {
			a.Add(val)
		}
	}
	want := map[string]string{"first": "decimal(3)", "last": "decimal(5)", "lowest": "decimal(2)", "highest": "decimal(5)",
		"total": "decimal(10)", "mean": "decimal(3.3333333333333333333333333333333333)", "count": "integer(3)"}
	for kind, a := range acc {
		if got := a.Result().String(); got != want[kind] {
			t.Errorf("%s = %s, want %s", kind, got, want[kind])
		}
	}
	if !acc["first"].Done() || acc["last"].Done() {
		t.Error("Done")
	}
	if got := NewAccumulator(total).Result(); !got.IsEmpty() {
		t.Errorf("total without values = %s", got)
	}

	for _, c := range []struct{ src, want string }{
		{"lowest(lowest([price]))", "aggregates cannot be nested"},
		{"total([name])", "expected a number"},
		{"highest([ok])", "true and false have no order"},
		{"first(value)", "has no source"},
		{"count(1, 2)", "count takes at most 1 argument"},
	} {
		if _, err := Compile(c.src, testScope{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.src, err, c.want)
		}
	}
	if p, _ := Compile("row() + first(row())", testScope{}); !p.UsesRow {
		t.Error("UsesRow")
	}
	if p, _ := Compile("first(row())", testScope{}); p.UsesRow {
		t.Error("row() inside an aggregate counted as a row reference")
	}
}
