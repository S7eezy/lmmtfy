package csvio

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

var comma = Dialect{Delimiter: ",", Quote: `"`, Escape: EscapeDouble}

func readAll(t *testing.T, input string, opts ReaderOptions) []Record {
	t.Helper()
	r, err := NewReader(strings.NewReader(input), opts)
	if err != nil {
		t.Fatal(err)
	}
	var out []Record
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rec)
	}
}

func fields(recs []Record) [][]string {
	var out [][]string
	for _, r := range recs {
		out = append(out, r.Fields)
	}
	return out
}

func TestReader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		opts  ReaderOptions
		want  [][]string
	}{
		{"simple", "a,b\n1,2\n", ReaderOptions{Dialect: comma}, [][]string{{"a", "b"}, {"1", "2"}}},
		{"crlf and no final newline", "a,b\r\n1,2", ReaderOptions{Dialect: comma}, [][]string{{"a", "b"}, {"1", "2"}}},
		{"empty fields", ",,\n", ReaderOptions{Dialect: comma}, [][]string{{"", "", ""}}},
		{"blank lines skipped", "a\n\n\r\nb\n", ReaderOptions{Dialect: comma}, [][]string{{"a"}, {"b"}}},
		{"quoted delimiter", `"a,b",c` + "\n", ReaderOptions{Dialect: comma}, [][]string{{"a,b", "c"}}},
		{"doubled quote", `"say ""hi""",x` + "\n", ReaderOptions{Dialect: comma}, [][]string{{`say "hi"`, "x"}}},
		{"multi-line field", "\"line1\nline2\",x\ny,z\n", ReaderOptions{Dialect: comma}, [][]string{{"line1\nline2", "x"}, {"y", "z"}}},
		{"text after closing quote kept", `"ab"c,d` + "\n", ReaderOptions{Dialect: comma}, [][]string{{"abc", "d"}}},
		{"quote inside unquoted field", `a"b,c` + "\n", ReaderOptions{Dialect: comma}, [][]string{{`a"b`, "c"}}},
		{
			"backslash escape", `"a\"b","c\\d"` + "\n",
			ReaderOptions{Dialect: Dialect{Delimiter: ",", Quote: `"`, Escape: EscapeBackslash}},
			[][]string{{`a"b`, `c\d`}},
		},
		{"semicolon", "a;b,c\n", ReaderOptions{Dialect: Dialect{Delimiter: ";", Quote: `"`, Escape: EscapeDouble}}, [][]string{{"a", "b,c"}}},
		{"tab no quoting", "a\t\"b\"\n", ReaderOptions{Dialect: Dialect{Delimiter: "\t", Escape: EscapeNone}}, [][]string{{"a", `"b"`}}},
		{"skip lines and footer", "junk\n\"more\njunk\na,b\n1,2\n3,4\nTotal: 2\n", ReaderOptions{Dialect: comma, SkipLines: 3, SkipFooter: 1}, [][]string{{"a", "b"}, {"1", "2"}, {"3", "4"}}},
		{"comments", "# note\na\n#x\nb\n", ReaderOptions{Dialect: comma, CommentPrefix: "#"}, [][]string{{"a"}, {"b"}}},
		{"utf-8 bom removed", "\xEF\xBB\xBFid,x\n", ReaderOptions{Dialect: comma}, [][]string{{"id", "x"}}},
		{"utf-8 bom kept", "\xEF\xBB\xBFid\n", ReaderOptions{Dialect: comma, BOM: BOMFalse}, [][]string{{"\uFEFFid"}}},
		{"latin-1", "N\xB0;Montant\n", ReaderOptions{Dialect: Dialect{Delimiter: ";", Quote: `"`}, Encoding: "latin-1"}, [][]string{{"N°", "Montant"}}},
		{"utf-16 with bom", "\xFF\xFEa\x00,\x00b\x00\n\x00", ReaderOptions{Dialect: comma, Encoding: "utf-16"}, [][]string{{"a", "b"}}},
		{"utf-16be from bom", "\xFE\xFF\x00a\x00,\x00b\x00\n", ReaderOptions{Dialect: comma, Encoding: "utf-16"}, [][]string{{"a", "b"}}},
		{"unicode delimiter", "a¦b\n", ReaderOptions{Dialect: Dialect{Delimiter: "¦"}}, [][]string{{"a", "b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fields(readAll(t, tt.input, tt.opts))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestReaderLineNumbers(t *testing.T) {
	recs := readAll(t, "a\n\n\"b\nc\"\nd\n", ReaderOptions{Dialect: comma})
	var lines []int
	for _, r := range recs {
		lines = append(lines, r.Line)
	}
	if !reflect.DeepEqual(lines, []int{1, 3, 5}) {
		t.Errorf("lines = %v, want [1 3 5]", lines)
	}
}

func TestReaderUnclosedQuote(t *testing.T) {
	r, _ := NewReader(strings.NewReader("a\n\"b,c\nd\n"), ReaderOptions{Dialect: comma})
	r.Read()
	_, err := r.Read()
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Line != 2 {
		t.Errorf("err = %v, want ParseError at line 2", err)
	}
}

func TestReaderLongLine(t *testing.T) {
	long := strings.Repeat("x", 3*bufferSize)
	got := fields(readAll(t, long+",y\n", ReaderOptions{Dialect: comma}))
	if len(got) != 1 || got[0][0] != long || got[0][1] != "y" {
		t.Errorf("long line not read correctly")
	}
}

func write(t *testing.T, opts WriterOptions, rows ...[]string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), nil
}

func TestWriter(t *testing.T) {
	base := WriterOptions{Dialect: comma, QuoteStyle: QuoteMinimal, LineEnding: "\r\n"}
	with := func(f func(*WriterOptions)) WriterOptions { o := base; f(&o); return o }

	tests := []struct {
		name string
		opts WriterOptions
		row  []string
		want string
	}{
		{"minimal", base, []string{"a", "b,c", `d"e`, "f\ng"}, "a,\"b,c\",\"d\"\"e\",\"f\ng\"\r\n"},
		{"always", with(func(o *WriterOptions) { o.QuoteStyle = QuoteAlways }), []string{"a", ""}, "\"a\",\"\"\r\n"},
		{"non numeric", with(func(o *WriterOptions) { o.QuoteStyle = QuoteNonNumeric; o.Numeric = []bool{true, false} }), []string{"12", "x"}, "12,\"x\"\r\n"},
		{"backslash", with(func(o *WriterOptions) { o.Escape = EscapeBackslash }), []string{`a"b`, `c\d`}, "\"a\\\"b\",\"c\\\\d\"\r\n"},
		{"lf semicolon", with(func(o *WriterOptions) { o.Delimiter = ";"; o.LineEnding = "\n" }), []string{"a,b", "c"}, "a,b;c\n"},
		{"utf-8 bom", with(func(o *WriterOptions) { o.BOM = true }), []string{"é"}, "\xEF\xBB\xBFé\r\n"},
		{"latin-1", with(func(o *WriterOptions) { o.Encoding = "latin-1" }), []string{"N°"}, "N\xB0\r\n"},
		{"utf-16le bom", with(func(o *WriterOptions) { o.Encoding = "utf-16le"; o.BOM = true; o.LineEnding = "\n" }), []string{"a"}, "\xFF\xFEa\x00\n\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := write(t, tt.opts, tt.row)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestWriterRowErrors(t *testing.T) {
	base := WriterOptions{Dialect: comma, QuoteStyle: QuoteNever, LineEnding: "\n"}
	_, err := write(t, base, []string{"a,b"})
	var re *RowError
	if !errors.As(err, &re) || re.Field != 0 {
		t.Errorf("quote never: err = %v", err)
	}

	latin := WriterOptions{Dialect: comma, QuoteStyle: QuoteMinimal, LineEnding: "\n", Encoding: "latin-1"}
	var buf bytes.Buffer
	w, _ := NewWriter(&buf, latin)
	if err := w.Write([]string{"€"}); !errors.As(err, &re) {
		t.Errorf("€ in latin-1: err = %v", err)
	}
	if err := w.Write([]string{"ok"}); err != nil {
		t.Errorf("writer unusable after RowError: %v", err)
	}
	w.Flush()
	if buf.String() != "ok\n" {
		t.Errorf("output = %q, want only the valid row", buf.String())
	}
}

func TestRoundTrip(t *testing.T) {
	rows := [][]string{{"a", "b,c", `"q"`, "line\r\nbreak", ""}, {"é", "€", "\t", " x ", `\`}}
	for _, d := range []Dialect{comma, {Delimiter: ";", Quote: "'", Escape: EscapeBackslash}} {
		out, err := write(t, WriterOptions{Dialect: d, QuoteStyle: QuoteMinimal, LineEnding: "\n"}, rows...)
		if err != nil {
			t.Fatal(err)
		}
		got := fields(readAll(t, out, ReaderOptions{Dialect: d}))
		if !reflect.DeepEqual(got, rows) {
			t.Errorf("dialect %+v:\ngot  %q\nwant %q", d, got, rows)
		}
	}
}

func BenchmarkReader(b *testing.B) {
	var sb strings.Builder
	for i := range 10000 {
		sb.WriteString(`12345,"Some text, with comma",2024-01-15,1234.56,`)
		sb.WriteString(strings.Repeat("x", i%40))
		sb.WriteString("\n")
	}
	data := sb.String()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		r, _ := NewReader(strings.NewReader(data), ReaderOptions{Dialect: comma})
		for {
			if _, err := r.Read(); err != nil {
				break
			}
		}
	}
}
