package csvio

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"
)

// Encoding is a text encoding. A nil Enc means UTF-8, handled natively
// without any transformation.
type Encoding struct {
	Name string
	Enc  encoding.Encoding
	// UTF16 is set for UTF-16 encodings, whose byte order can come from a BOM.
	UTF16     bool
	BigEndian bool
}

// LookupEncoding finds an encoding by name: utf-8, utf-16le, utf-16be,
// utf-16 (byte order from BOM, little-endian without), latin-1,
// iso-8859-15, windows-1252, or any IANA name such as windows-1250 or
// ibm850.
func LookupEncoding(name string) (Encoding, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "", "utf-8", "utf8":
		return Encoding{Name: "utf-8"}, nil
	case "utf-16", "utf16", "utf-16le", "utf16le":
		return Encoding{Name: key, Enc: unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), UTF16: true}, nil
	case "utf-16be", "utf16be":
		return Encoding{Name: key, Enc: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), UTF16: true, BigEndian: true}, nil
	case "latin-1", "latin1", "iso-8859-1", "iso8859-1":
		return Encoding{Name: key, Enc: charmap.ISO8859_1}, nil
	case "latin-9", "latin9", "iso-8859-15", "iso8859-15":
		return Encoding{Name: key, Enc: charmap.ISO8859_15}, nil
	case "windows-1252", "cp1252":
		return Encoding{Name: key, Enc: charmap.Windows1252}, nil
	}
	enc, err := ianaindex.IANA.Encoding(key)
	if err != nil || enc == nil {
		return Encoding{}, fmt.Errorf("unknown or unsupported encoding %q", name)
	}
	return Encoding{Name: key, Enc: enc}, nil
}

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// bom returns the byte order mark of the encoding, or nil if it has none.
func (e Encoding) bom() []byte {
	switch {
	case e.Enc == nil:
		return bomUTF8
	case e.UTF16 && e.BigEndian:
		return bomUTF16BE
	case e.UTF16:
		return bomUTF16LE
	}
	return nil
}
