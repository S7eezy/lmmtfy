package csvio

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Escape is how a quote character is written inside a quoted field.
type Escape string

const (
	EscapeDouble    Escape = "double"    // "" inside quotes is a literal "
	EscapeBackslash Escape = "backslash" // \" and \\ inside quotes
	EscapeNone      Escape = "none"      // a quote always ends the field
)

// Dialect is the shape of a CSV file.
type Dialect struct {
	Delimiter string // one character
	Quote     string // one character; "" disables quoting
	Escape    Escape
}

// BOM handling when reading.
const (
	BOMAuto  = "auto"  // remove a BOM if present (and use its byte order for UTF-16)
	BOMTrue  = "true"  // same as auto: a missing BOM is not an error
	BOMFalse = "false" // keep the bytes: a BOM becomes part of the first value
)

// ReaderOptions configures a Reader.
type ReaderOptions struct {
	Dialect
	Encoding      string
	BOM           string
	SkipLines     int    // physical lines skipped before the first record
	CommentPrefix string // records whose line starts with this are ignored
	SkipFooter    int    // records dropped at the end of the file
}

// Record is one CSV record.
type Record struct {
	Line   int // physical line where the record starts, 1-based
	Fields []string
}

// ParseError is a malformed file. Reading cannot continue after it, since
// record boundaries are no longer known.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// Reader reads records one at a time, with constant memory.
//
// Lines end with \n or \r\n. Blank lines are skipped. Text after a closing
// quote and before the next delimiter is kept as part of the field ("ab"c,
// reads as abc): messy files do this and rejecting them helps nobody.
type Reader struct {
	br      *bufio.Reader
	opts    ReaderOptions
	delim   []byte
	quote   []byte
	comment []byte

	line    int    // physical lines consumed
	lineBuf []byte // used when a line exceeds the bufio buffer
	field   []byte // current record's field bytes, concatenated
	ends    []int  // end offset of each field in field
	queue   []Record
	started bool
	eof     bool
}

const bufferSize = 256 * 1024

// NewReader returns a reader decoding src with the given options.
func NewReader(src io.Reader, opts ReaderOptions) (*Reader, error) {
	enc, err := LookupEncoding(opts.Encoding)
	if err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(opts.Delimiter) != 1 {
		return nil, fmt.Errorf("delimiter must be a single character, got %q", opts.Delimiter)
	}

	br := bufio.NewReaderSize(src, bufferSize)
	if opts.BOM != BOMFalse {
		if err := stripBOM(br, &enc); err != nil {
			return nil, err
		}
	}
	if enc.Enc != nil {
		br = bufio.NewReaderSize(transform.NewReader(br, enc.Enc.NewDecoder()), bufferSize)
	}

	return &Reader{
		br:      br,
		opts:    opts,
		delim:   []byte(opts.Delimiter),
		quote:   []byte(opts.Quote),
		comment: []byte(opts.CommentPrefix),
	}, nil
}

// stripBOM removes a leading BOM. For UTF-16 the BOM decides the byte order.
func stripBOM(br *bufio.Reader, enc *Encoding) error {
	head, err := br.Peek(3)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return err
	}
	switch {
	case enc.Enc == nil && bytes.HasPrefix(head, bomUTF8):
		_, err = br.Discard(3)
	case enc.UTF16 && bytes.HasPrefix(head, bomUTF16LE):
		enc.Enc, enc.BigEndian = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), false
		_, err = br.Discard(2)
	case enc.UTF16 && bytes.HasPrefix(head, bomUTF16BE):
		enc.Enc, enc.BigEndian = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), true
		_, err = br.Discard(2)
	}
	return err
}

// Read returns the next record, or io.EOF at the end of the input.
func (r *Reader) Read() (Record, error) {
	if !r.started {
		r.started = true
		for range r.opts.SkipLines {
			if _, err := r.readLine(); err != nil {
				return Record{}, err
			}
		}
	}

	// Keep SkipFooter records in reserve: they are only known to be the
	// footer once the end of the file is reached.
	for len(r.queue) <= r.opts.SkipFooter && !r.eof {
		rec, err := r.readRecord()
		if errors.Is(err, io.EOF) {
			r.eof = true
			break
		}
		if err != nil {
			return Record{}, err
		}
		r.queue = append(r.queue, rec)
	}
	if len(r.queue) > r.opts.SkipFooter {
		rec := r.queue[0]
		r.queue = r.queue[1:]
		return rec, nil
	}
	return Record{}, io.EOF
}

// readLine returns the next physical line, line ending included. The slice
// is only valid until the next call.
func (r *Reader) readLine() ([]byte, error) {
	line, err := r.br.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		r.lineBuf = append(r.lineBuf[:0], line...)
		for errors.Is(err, bufio.ErrBufferFull) {
			line, err = r.br.ReadSlice('\n')
			r.lineBuf = append(r.lineBuf, line...)
		}
		line = r.lineBuf
	}
	if errors.Is(err, io.EOF) && len(line) > 0 {
		err = nil // last line without line ending
	}
	if err != nil {
		return nil, err
	}
	r.line++
	return line, nil
}

func (r *Reader) readRecord() (Record, error) {
	for {
		line, err := r.readLine()
		if err != nil {
			return Record{}, err
		}
		if len(trimEOL(line)) == 0 {
			continue
		}
		if len(r.comment) > 0 && bytes.HasPrefix(line, r.comment) {
			continue
		}
		return r.parseRecord(line, r.line)
	}
}

func (r *Reader) parseRecord(line []byte, start int) (Record, error) {
	r.field = r.field[:0]
	r.ends = r.ends[:0]
	pos := 0

	for {
		if len(r.quote) > 0 && bytes.HasPrefix(line[pos:], r.quote) {
			// Quoted field, possibly spanning several lines.
			pos += len(r.quote)
		quoted:
			for {
				rest := line[pos:]
				qi := bytes.Index(rest, r.quote)
				bi := -1
				if r.opts.Escape == EscapeBackslash {
					bi = bytes.IndexByte(rest, '\\')
				}
				switch {
				case bi >= 0 && (qi < 0 || bi < qi):
					r.field = append(r.field, rest[:bi]...)
					if bi+1 >= len(rest) { // backslash at the very end of the input
						r.field = append(r.field, '\\')
						pos += bi + 1
						continue
					}
					_, size := utf8.DecodeRune(rest[bi+1:])
					r.field = append(r.field, rest[bi+1:bi+1+size]...)
					pos += bi + 1 + size
				case qi >= 0:
					r.field = append(r.field, rest[:qi]...)
					pos += qi + len(r.quote)
					if r.opts.Escape == EscapeDouble && bytes.HasPrefix(line[pos:], r.quote) {
						r.field = append(r.field, r.quote...)
						pos += len(r.quote)
						continue
					}
					break quoted
				default:
					// No closing quote on this line: the field goes on.
					r.field = append(r.field, rest...)
					next, err := r.readLine()
					if errors.Is(err, io.EOF) {
						return Record{}, &ParseError{Line: start, Msg: "quoted field is never closed"}
					}
					if err != nil {
						return Record{}, err
					}
					line, pos = next, 0
				}
			}
		}

		rest := line[pos:]
		di := bytes.Index(rest, r.delim)
		if di < 0 {
			r.field = append(r.field, trimEOL(rest)...)
			r.ends = append(r.ends, len(r.field))
			break
		}
		r.field = append(r.field, rest[:di]...)
		r.ends = append(r.ends, len(r.field))
		pos += di + len(r.delim)
	}

	// One string per record, sliced into fields: one allocation, not one per field.
	all := string(r.field)
	fields := make([]string, len(r.ends))
	begin := 0
	for i, end := range r.ends {
		fields[i] = all[begin:end]
		begin = end
	}
	return Record{Line: start, Fields: fields}, nil
}

func trimEOL(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte("\n"))
	return bytes.TrimSuffix(b, []byte("\r"))
}
