package csvio

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

var fuzzDialects = []Dialect{
	{Delimiter: ",", Quote: `"`, Escape: EscapeDouble},
	{Delimiter: ";", Quote: `"`, Escape: EscapeBackslash},
	{Delimiter: "\t", Quote: "'", Escape: EscapeNone},
	{Delimiter: "|"},
}

// FuzzReader feeds arbitrary bytes to the reader: it must never panic, and
// whatever it reads must survive a write/read round trip.
func FuzzReader(f *testing.F) {
	for _, s := range []string{"a,b\n1,2\n", "\"a\"\"b\",c\r\n", "\"unclosed\n", "\\\"x\\\"", "\xEF\xBB\xBFa;b", "a\tb\n\n", "'q''x'"} {
		f.Add([]byte(s), uint8(0))
	}
	f.Fuzz(func(t *testing.T, data []byte, which uint8) {
		d := fuzzDialects[int(which)%len(fuzzDialects)]
		recs := readAllBytes(data, ReaderOptions{Dialect: d, SkipFooter: int(which) % 3})

		// A record of one empty field is written as an empty line, which
		// readers skip: leave those out of the round trip.
		var rows [][]string
		for _, r := range recs {
			if len(r) == 1 && r[0] == "" {
				continue
			}
			rows = append(rows, r)
		}
		if d.Quote == "" || len(rows) == 0 {
			return
		}
		// U+FEFF at the start of a file is a byte order mark, so a first
		// value starting with it cannot survive a round trip.
		if strings.HasPrefix(rows[0][0], "\ufeff") {
			return
		}
		wd := d
		if wd.Escape == EscapeNone {
			wd.Escape = EscapeDouble
		}
		var buf bytes.Buffer
		w, err := NewWriter(&buf, WriterOptions{Dialect: wd, QuoteStyle: QuoteMinimal, LineEnding: "\n"})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if err := w.Write(r); err != nil {
				t.Fatalf("write %q: %v", r, err)
			}
		}
		w.Flush()
		back := readAllBytes(buf.Bytes(), ReaderOptions{Dialect: wd})
		if !reflect.DeepEqual(back, rows) {
			t.Fatalf("round trip:\nwrote %q\nread  %q\nfile  %q", rows, back, buf.String())
		}
	})
}

func readAllBytes(data []byte, opts ReaderOptions) [][]string {
	r, err := NewReader(bytes.NewReader(data), opts)
	if err != nil {
		return nil
	}
	var out [][]string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) || err != nil {
			return out
		}
		out = append(out, rec.Fields)
	}
}
