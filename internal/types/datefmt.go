package types

import (
	"fmt"
	"strings"
)

// Date format tokens, longest first so that "MMMM" wins over "MM".
// Values are Go reference-time layout elements (Mon Jan 2 15:04:05 2006).
var dateTokens = []struct{ token, layout string }{
	{"YYYY", "2006"},
	{"YY", "06"},
	{"MMMM", "January"},
	{"MMM", "Jan"},
	{"MM", "01"},
	{"M", "1"},
	{"DD", "02"},
	{"D", "2"},
	{"HH", "15"},
	{"hh", "03"},
	{"h", "3"},
	{"mm", "04"},
	{"m", "4"},
	{"ss", "05"},
	{"s", "5"},
	{"SSSSSSSSS", "000000000"},
	{"SSSSSS", "000000"},
	{"SSS", "000"},
	{"A", "PM"},
	{"a", "pm"},
	{"ZZ", "-0700"},
	{"Z", "Z07:00"},
}

// Layout translates a human-readable date format ("DD/MM/YYYY HH:mm") into a
// Go time layout ("02/01/2006 15:04").
//
// Supported tokens: YYYY YY MMMM MMM MM M DD D HH hh h mm m ss s SSS SSSSSS
// SSSSSSSSS A a ZZ Z. Fractional seconds (SSS...) must follow "." or ",".
// Punctuation, spaces and the letter T are kept literally. Any other letter
// or digit is rejected: Go would silently read it as a layout element.
//
// Single-letter tokens (M, D, h, m, s) accept one or two digits when parsing;
// double-letter ones require two (except HH, which Go always parses leniently).
func Layout(format string) (string, error) {
	if format == "" {
		return "", fmt.Errorf("empty date format")
	}
	var b strings.Builder
	rest := format
	for rest != "" {
		matched := false
		for _, tok := range dateTokens {
			if !strings.HasPrefix(rest, tok.token) {
				continue
			}
			if tok.token[0] == 'S' {
				prev := b.String()
				if !strings.HasSuffix(prev, ".") && !strings.HasSuffix(prev, ",") {
					return "", fmt.Errorf("format %q: %s must follow \".\" or \",\"", format, tok.token)
				}
			}
			b.WriteString(tok.layout)
			rest = rest[len(tok.token):]
			matched = true
			break
		}
		if matched {
			continue
		}

		c := rest[0]
		switch {
		case c == 'T':
		case c >= '0' && c <= '9':
			return "", fmt.Errorf("format %q: digits are not allowed (use tokens such as YYYY, MM, DD)", format)
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			return "", fmt.Errorf("format %q: unknown token starting at %q", format, rest)
		case c == '_':
			return "", fmt.Errorf("format %q: \"_\" is not allowed", format)
		}
		b.WriteByte(c)
		rest = rest[1:]
	}
	return b.String(), nil
}
