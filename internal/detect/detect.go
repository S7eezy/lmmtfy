package detect

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/engine"
)

// Options tunes detection.
type Options struct {
	SampleBytes int     // bytes read from the start of the file; default 4 MiB
	SampleRows  int     // data rows analysed; default 10,000
	Threshold   float64 // share of values a type must accept; default 0.95
}

func (o *Options) defaults() {
	if o.SampleBytes <= 0 {
		o.SampleBytes = 4 << 20
	}
	if o.SampleRows <= 0 {
		o.SampleRows = 10000
	}
	if o.Threshold <= 0 || o.Threshold > 1 {
		o.Threshold = 0.95
	}
}

// Result is what detection found: input options for a mapping, and one
// proposal per column.
type Result struct {
	Encoding         string   `json:"encoding"`
	BOM              bool     `json:"bom"`
	Delimiter        string   `json:"delimiter"`
	Quote            string   `json:"quote"`
	Escape           string   `json:"escape"`
	Header           bool     `json:"header"`
	HeaderRow        int      `json:"header_row"` // line of the header, or of the first row without header
	SkipFooter       int      `json:"skip_footer"`
	Trim             bool     `json:"trim"`
	NullTokens       []string `json:"null_tokens,omitempty"`
	RaggedRows       string   `json:"ragged_rows"`
	DuplicateHeaders bool     `json:"duplicate_headers"`

	Columns    []Column `json:"columns"`
	SampleRows int      `json:"sample_rows"`
	WholeFile  bool     `json:"whole_file"` // the sample covers the whole file
	Notes      []string `json:"notes,omitempty"`
}

// tailBytes is how much of the end of a large file is read to find a footer.
const tailBytes = 64 << 10

// File detects the format of the file at path. It reads the start of the
// file and, for files larger than the sample, its last bytes.
func File(path string, opts Options) (*Result, error) {
	opts.defaults()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	sample := make([]byte, min(int64(opts.SampleBytes), info.Size()))
	if _, err := io.ReadFull(f, sample); err != nil {
		return nil, err
	}
	var tail []byte
	if rest := info.Size() - int64(len(sample)); rest > 0 {
		tail = make([]byte, min(rest, tailBytes))
		if _, err := f.ReadAt(tail, info.Size()-int64(len(tail))); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	return Detect(sample, tail, tail != nil, opts)
}

// Detect works on the first bytes of a file and, when they do not reach the
// end (truncated), its last bytes.
func Detect(sample, tail []byte, truncated bool, opts Options) (*Result, error) {
	opts.defaults()
	if len(bytes.TrimSpace(sample)) == 0 {
		return nil, errors.New("file is empty")
	}

	res := &Result{Quote: `"`, Escape: "double", RaggedRows: "error", WholeFile: !truncated}
	res.Encoding, res.BOM = detectEncoding(sample)
	nl := newline(res.Encoding)
	if truncated {
		// Drop the last, probably incomplete, line.
		if i := lastIndexAligned(sample, nl); i >= 0 {
			sample = sample[:i+len(nl)]
		}
		// The tail starts mid-line: skip to the first full line.
		if i := indexAligned(tail, nl, len(tail)); i >= 0 {
			tail = tail[i+len(nl):]
		}
	}
	if res.Encoding == "windows-1252" || res.Encoding == "utf-8" {
		if bytes.Contains(sample, []byte(`\"`)) {
			res.Escape = "backslash"
		}
	}

	res.Delimiter = detectDelimiter(sample, res)
	recs := parse(sample, res, opts.SampleRows+50)
	if len(recs) == 0 {
		return nil, errors.New("no rows found")
	}
	width := modeWidth(recs)

	// Header: first row with the usual field count where most cells are filled.
	// Rows before it (titles, export dates) are a preamble.
	h := slices.IndexFunc(recs, func(r csvio.Record) bool { return len(r.Fields) == width && filled(r.Fields) > 0.5 })
	if h < 0 {
		h = slices.IndexFunc(recs, func(r csvio.Record) bool { return len(r.Fields) == width })
	}
	res.HeaderRow = recs[h].Line
	candidate := recs[h]
	data := recs[h+1:]
	if len(data) > opts.SampleRows {
		data = data[:opts.SampleRows]
	}

	res.Trim = needsTrim(data)
	res.SkipFooter = detectFooter(data, tail, truncated, width, res, opts)
	if !truncated {
		data = data[:len(data)-res.SkipFooter]
	}

	// Without header, the candidate row is data: it parses like the rows below.
	cols := inferColumns(data, width, res.Trim, opts)
	res.Header = !looksLikeData(candidate, cols, res.Trim)
	var names []string
	if res.Header {
		names = candidate.Fields
	} else {
		data = append([]csvio.Record{candidate}, data...)
	}
	cols = finalColumns(names, data, width, res, opts)
	res.Columns = cols
	res.SampleRows = len(data)

	short, long := 0, 0
	for _, r := range data {
		switch {
		case len(r.Fields) < width:
			short++
		case len(r.Fields) > width:
			long++
		}
	}
	switch {
	case short > 0 && long > 0:
		res.RaggedRows = "lenient"
	case short > 0:
		res.RaggedRows = "pad"
	case long > 0:
		res.RaggedRows = "truncate"
	}
	if short+long > 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("%d of %d sampled rows do not have %d fields: ragged_rows set to %s", short+long, len(data), width, res.RaggedRows))
	}
	if !res.Header && !slices.ContainsFunc(cols, func(c Column) bool { return c.Type != "string" }) {
		res.Header = true
		res.Notes = append(res.Notes, "all columns are text: the first row is assumed to be a header")
	}
	if res.Header {
		if _, dup := engine.NormalizeHeader(candidate.Fields); dup != "" {
			res.DuplicateHeaders = true
			res.Notes = append(res.Notes, "duplicate header names: they are numbered (Name, Name_2, ...)")
		}
	}
	if truncated {
		res.Notes = append(res.Notes, fmt.Sprintf("file larger than the sample: types are based on the first %d rows", len(data)))
	}
	return res, nil
}

// detectEncoding guesses the encoding from a BOM, from the zero bytes of
// UTF-16 text, or from UTF-8 validity. Anything else is taken as
// windows-1252, a superset of latin-1 used by most Windows exports.
func detectEncoding(b []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return "utf-8", true
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return "utf-16le", true
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return "utf-16be", true
	}

	// ASCII-range text in UTF-16 has a zero byte in every other position.
	head := b[:min(len(b), 2000)]
	var even, odd int
	for i, c := range head {
		if c == 0 {
			if i%2 == 0 {
				even++
			} else {
				odd++
			}
		}
	}
	half := len(head) / 2
	switch {
	case half > 0 && odd > half*4/10 && even <= half/20:
		return "utf-16le", false
	case half > 0 && even > half*4/10 && odd <= half/20:
		return "utf-16be", false
	}

	// A truncated sample may end in the middle of a character.
	for cut := 0; cut <= 3 && cut < len(b); cut++ {
		if utf8.Valid(b[:len(b)-cut]) {
			return "utf-8", false
		}
	}
	return "windows-1252", false
}

func newline(encoding string) []byte {
	switch encoding {
	case "utf-16le":
		return []byte{'\n', 0}
	case "utf-16be":
		return []byte{0, '\n'}
	}
	return []byte{'\n'}
}

// lastIndexAligned finds the last nl starting at a multiple of len(nl), so a
// UTF-16 newline is not confused with the bytes of two other characters.
func lastIndexAligned(b, nl []byte) int {
	for i := len(b) - len(nl); i >= 0; i-- {
		if i%len(nl) == 0 && bytes.Equal(b[i:i+len(nl)], nl) {
			return i
		}
	}
	return -1
}

// indexAligned finds the first nl aligned on the end of a buffer of total
// length n (a tail read backwards from the end of the file).
func indexAligned(b, nl []byte, n int) int {
	for i := 0; i+len(nl) <= len(b); i++ {
		if (n-i)%len(nl) == 0 && bytes.Equal(b[i:i+len(nl)], nl) {
			return i
		}
	}
	return -1
}

// Delimiters in tie-break order: when two split the rows equally well
// ("1;2,50" is two fields either way), the earlier wins. Semicolon comes
// before comma because files using ";" usually do so to keep "," for
// decimals.
var delimiters = []string{"\t", ";", "|", ","}

// detectDelimiter picks the delimiter giving the most consistent field
// count over the first records, preferring more fields on ties.
func detectDelimiter(sample []byte, res *Result) string {
	best, bestScore, bestWidth := ",", -1.0, 0
	for _, d := range delimiters {
		trial := *res
		trial.Delimiter = d
		recs := parse(sample, &trial, 200)
		if len(recs) == 0 {
			continue
		}
		width := modeWidth(recs)
		if width < 2 {
			continue
		}
		same := 0
		for _, r := range recs {
			if len(r.Fields) == width {
				same++
			}
		}
		score := float64(same) / float64(len(recs))
		if score > bestScore+0.01 || score > bestScore-0.01 && width > bestWidth {
			best, bestScore, bestWidth = d, score, width
		}
	}
	return best
}

// parse reads up to max records. A parse error (often a sample cut inside a
// quoted field) ends the list instead of failing detection.
func parse(sample []byte, res *Result, max int) []csvio.Record {
	r, err := csvio.NewReader(bytes.NewReader(sample), csvio.ReaderOptions{
		Dialect:  csvio.Dialect{Delimiter: res.Delimiter, Quote: res.Quote, Escape: csvio.Escape(res.Escape)},
		Encoding: res.Encoding,
		BOM:      csvio.BOMAuto,
	})
	if err != nil {
		return nil
	}
	var recs []csvio.Record
	for len(recs) < max {
		rec, err := r.Read()
		if err != nil {
			break
		}
		recs = append(recs, rec)
	}
	return recs
}

func modeWidth(recs []csvio.Record) int {
	counts := map[int]int{}
	best, bestCount := 0, 0
	for _, r := range recs {
		n := len(r.Fields)
		counts[n]++
		if counts[n] > bestCount || counts[n] == bestCount && n > best {
			best, bestCount = n, counts[n]
		}
	}
	return best
}

func filled(fields []string) float64 {
	n := 0
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			n++
		}
	}
	return float64(n) / float64(len(fields))
}

func needsTrim(data []csvio.Record) bool {
	for _, r := range data {
		for _, f := range r.Fields {
			if f != "" && strings.TrimSpace(f) != f {
				return true
			}
		}
	}
	return false
}
