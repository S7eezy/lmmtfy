package csvio

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
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
	w *bufio.Writer
	f *Formatter
}

// Formatter turns records into encoded bytes. Formatting is the expensive
// part of writing; a Formatter per goroutine lets rows be formatted in
// parallel while one Writer writes them in order. A Formatter is not safe for
// concurrent use.
type Formatter struct {
	opts       WriterOptions
	enc        *encoding.Encoder
	name       string
	buf        []byte
	needsQuote string // characters that force quoting
	escaped    *strings.Replacer
}

// NewWriter returns a writer to dst. The BOM, if any, is written immediately.
func NewWriter(dst io.Writer, opts WriterOptions) (*Writer, error) {
	f, err := NewFormatter(opts)
	if err != nil {
		return nil, err
	}
	w := &Writer{w: bufio.NewWriterSize(dst, bufferSize), f: f}
	if opts.BOM {
		enc, _ := LookupEncoding(opts.Encoding) // already checked by NewFormatter
		if b := enc.bom(); b != nil {
			w.w.Write(b) // bufio.Writer: errors surface on Flush
		}
	}
	return w, nil
}

// NewFormatter returns a formatter for opts.
func NewFormatter(opts WriterOptions) (*Formatter, error) {
	enc, err := LookupEncoding(opts.Encoding)
	if err != nil {
		return nil, err
	}
	f := &Formatter{opts: opts, name: enc.Name}
	if enc.Enc != nil {
		f.enc = enc.Enc.NewEncoder()
	}
	f.needsQuote = opts.Delimiter + "\r\n" + opts.Quote
	switch opts.Escape {
	case EscapeBackslash:
		f.needsQuote += `\`
		f.escaped = strings.NewReplacer(`\`, `\\`, opts.Quote, `\`+opts.Quote)
	case EscapeDouble:
		if opts.Quote != "" {
			f.escaped = strings.NewReplacer(opts.Quote, opts.Quote+opts.Quote)
		}
	}
	return f, nil
}

// Write writes one record. A *RowError means the row was not written and the
// writer can still be used.
func (w *Writer) Write(fields []string) error {
	b, err := w.f.Append(nil, fields)
	if err != nil {
		return err
	}
	return w.WriteFormatted(b)
}

// WriteFormatted writes bytes produced by a Formatter with the same options.
func (w *Writer) WriteFormatted(b []byte) error {
	_, err := w.w.Write(b)
	return err
}

// Flush writes buffered data to the underlying writer.
func (w *Writer) Flush() error { return w.w.Flush() }

// Append appends the encoded record, line ending included, to dst. On a
// *RowError dst is returned unchanged.
func (f *Formatter) Append(dst []byte, fields []string) ([]byte, error) {
	// Without transcoding, format straight into dst; otherwise into a
	// scratch buffer that is then encoded into dst.
	b := dst
	if f.enc != nil {
		b = f.buf[:0]
	}
	start := len(b)
	for i, field := range fields {
		if i > 0 {
			b = append(b, f.opts.Delimiter...)
		}
		needs := strings.ContainsAny(field, f.needsQuote)
		quote := needs
		switch f.opts.QuoteStyle {
		case QuoteAlways:
			quote = true
		case QuoteNonNumeric:
			quote = needs || i >= len(f.opts.Numeric) || !f.opts.Numeric[i]
		case QuoteNever:
			quote = false
		}

		if needs && (!quote || f.opts.Quote == "") {
			return dst, &RowError{Field: i, Msg: "value contains a delimiter, quote or line break, but quoting is disabled"}
		}
		if needs && f.opts.Quote != "" && f.escaped == nil && strings.Contains(field, f.opts.Quote) {
			return dst, &RowError{Field: i, Msg: "value contains the quote character, but escape is none"}
		}

		if quote && f.opts.Quote != "" {
			b = append(b, f.opts.Quote...)
			if f.escaped != nil && strings.ContainsAny(field, f.needsQuote) {
				field = f.escaped.Replace(field)
			}
			b = append(b, field...)
			b = append(b, f.opts.Quote...)
		} else {
			b = append(b, field...)
		}
	}
	b = append(b, f.opts.LineEnding...)

	if f.enc == nil {
		return b, nil
	}
	f.buf = b
	out, _, err := transform.Append(f.enc, dst, b[start:])
	if err != nil {
		f.enc.Reset()
		return dst, &RowError{Field: -1, Msg: fmt.Sprintf("row contains characters that cannot be written in %s", f.name)}
	}
	return out, nil
}
