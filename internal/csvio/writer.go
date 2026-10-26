package csvio

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding"
)

// QuoteStyle decides which fields are quoted.
type QuoteStyle string

const (
	QuoteMinimal    QuoteStyle = "minimal"     // only fields that need it
	QuoteAlways     QuoteStyle = "always"      // every field
	QuoteNever      QuoteStyle = "never"       // no field; a field that needs quoting is an error
	QuoteNonNumeric QuoteStyle = "non_numeric" // every field except numeric columns
)

// WriterOptions configures a Writer.
type WriterOptions struct {
	Dialect
	Encoding   string
	BOM        bool
	QuoteStyle QuoteStyle
	LineEnding string // "\r\n" or "\n"
	// Numeric marks numeric columns, for QuoteNonNumeric.
	Numeric []bool
}

// RowError is a row that cannot be written as configured. Nothing of the
// row has been written.
type RowError struct {
	Field int // -1 when not tied to one field
	Msg   string
}

func (e *RowError) Error() string { return e.Msg }

// Writer writes records with a configured dialect and encoding.
type Writer struct {
	w    *bufio.Writer
	opts WriterOptions
	enc  *encoding.Encoder
	name string
	buf  []byte

	needsQuote string // characters that force quoting
	escaped    *strings.Replacer
}

// NewWriter returns a writer to dst. The BOM, if any, is written immediately.
func NewWriter(dst io.Writer, opts WriterOptions) (*Writer, error) {
	enc, err := LookupEncoding(opts.Encoding)
	if err != nil {
		return nil, err
	}
	w := &Writer{w: bufio.NewWriterSize(dst, bufferSize), opts: opts, name: enc.Name}
	if enc.Enc != nil {
		w.enc = enc.Enc.NewEncoder()
	}
	if opts.BOM {
		if b := enc.bom(); b != nil {
			w.w.Write(b) // bufio.Writer: errors surface on Flush
		}
	}

	w.needsQuote = opts.Delimiter + "\r\n" + opts.Quote
	switch opts.Escape {
	case EscapeBackslash:
		w.needsQuote += `\`
		w.escaped = strings.NewReplacer(`\`, `\\`, opts.Quote, `\`+opts.Quote)
	case EscapeDouble:
		if opts.Quote != "" {
			w.escaped = strings.NewReplacer(opts.Quote, opts.Quote+opts.Quote)
		}
	}
	return w, nil
}

// Write writes one record. A *RowError means the row was not written and the
// writer can still be used.
func (w *Writer) Write(fields []string) error {
	w.buf = w.buf[:0]
	for i, f := range fields {
		if i > 0 {
			w.buf = append(w.buf, w.opts.Delimiter...)
		}
		needs := strings.ContainsAny(f, w.needsQuote)
		quote := needs
		switch w.opts.QuoteStyle {
		case QuoteAlways:
			quote = true
		case QuoteNonNumeric:
			quote = needs || i >= len(w.opts.Numeric) || !w.opts.Numeric[i]
		case QuoteNever:
			quote = false
		}

		if needs && (!quote || w.opts.Quote == "") {
			return &RowError{Field: i, Msg: "value contains a delimiter, quote or line break, but quoting is disabled"}
		}
		if needs && w.opts.Quote != "" && w.escaped == nil && strings.Contains(f, w.opts.Quote) {
			return &RowError{Field: i, Msg: "value contains the quote character, but escape is none"}
		}

		if quote && w.opts.Quote != "" {
			w.buf = append(w.buf, w.opts.Quote...)
			if w.escaped != nil {
				f = w.escaped.Replace(f)
			}
			w.buf = append(w.buf, f...)
			w.buf = append(w.buf, w.opts.Quote...)
		} else {
			w.buf = append(w.buf, f...)
		}
	}
	w.buf = append(w.buf, w.opts.LineEnding...)

	out := w.buf
	if w.enc != nil {
		encoded, err := w.enc.Bytes(w.buf)
		if err != nil {
			return &RowError{Field: -1, Msg: fmt.Sprintf("row contains characters that cannot be written in %s", w.name)}
		}
		out = encoded
	}
	_, err := w.w.Write(out)
	return err
}

// Flush writes buffered data to the underlying writer.
func (w *Writer) Flush() error { return w.w.Flush() }
