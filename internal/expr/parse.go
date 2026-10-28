package expr

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Error is a problem in a formula. Pos is the 1-based character where it
// is, 0 when it concerns the whole formula.
type Error struct {
	Pos int
	Msg string
}

func (e *Error) Error() string {
	if e.Pos == 0 {
		return e.Msg
	}
	return fmt.Sprintf("%s (character %d)", e.Msg, e.Pos)
}

func errorAt(pos int, format string, args ...any) *Error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokString
	tokColumn
	tokName
	tokOp
)

type token struct {
	kind tokenKind
	text string // number digits, unquoted string, column name, name or operator
	pos  int    // 1-based character
}

// tokenize splits a formula into tokens.
func tokenize(src string) ([]token, error) {
	var toks []token
	pos := 1 // in characters
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRuneInString(src[i:])
		start := pos
		switch {
		case unicode.IsSpace(r):
			i += size
			pos++
			continue

		case r >= '0' && r <= '9':
			j := i
			for j < len(src) && isDigit(src[j]) {
				j++
			}
			if j+1 < len(src) && src[j] == '.' && isDigit(src[j+1]) {
				j++
				for j < len(src) && isDigit(src[j]) {
					j++
				}
			}
			toks = append(toks, token{tokNumber, src[i:j], start})
			pos += j - i
			i = j
			continue

		case r == '"':
			// "" inside a string is a quote, as in spreadsheets.
			var b strings.Builder
			j := i + 1
			n := 1
			for {
				if j >= len(src) {
					return nil, errorAt(start, "text is not closed: a \" is missing")
				}
				c, sz := utf8.DecodeRuneInString(src[j:])
				if c == '"' {
					if j+1 < len(src) && src[j+1] == '"' {
						b.WriteByte('"')
						j += 2
						n += 2
						continue
					}
					j++
					n++
					break
				}
				b.WriteRune(c)
				j += sz
				n++
			}
			toks = append(toks, token{tokString, b.String(), start})
			pos += n
			i = j
			continue

		case r == '[':
			// ]] inside a column name is a ].
			var b strings.Builder
			j := i + 1
			n := 1
			for {
				if j >= len(src) {
					return nil, errorAt(start, "column name is not closed: a ] is missing")
				}
				c, sz := utf8.DecodeRuneInString(src[j:])
				if c == ']' {
					if j+1 < len(src) && src[j+1] == ']' {
						b.WriteByte(']')
						j += 2
						n += 2
						continue
					}
					j++
					n++
					break
				}
				b.WriteRune(c)
				j += sz
				n++
			}
			name := strings.TrimSpace(b.String())
			if name == "" {
				return nil, errorAt(start, "empty column name")
			}
			toks = append(toks, token{tokColumn, name, start})
			pos += n
			i = j
			continue

		case r == '_' || unicode.IsLetter(r):
			j := i
			n := 0
			for j < len(src) {
				c, sz := utf8.DecodeRuneInString(src[j:])
				if c != '_' && !unicode.IsLetter(c) && !unicode.IsDigit(c) {
					break
				}
				j += sz
				n++
			}
			toks = append(toks, token{tokName, strings.ToLower(src[i:j]), start})
			pos += n
			i = j
			continue
		}

		op := ""
		for _, o := range []string{"<>", "!=", "<=", ">=", "+", "-", "*", "/", "%", "&", "=", "<", ">", "(", ")", ","} {
			if strings.HasPrefix(src[i:], o) {
				op = o
				break
			}
		}
		if op == "" {
			return nil, errorAt(start, "unexpected %q", r)
		}
		if op == "!=" {
			op = "<>"
		}
		toks = append(toks, token{tokOp, op, start})
		i += len(op)
		pos += len(op)
	}
	return append(toks, token{tokEOF, "", pos}), nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// Syntax tree.
type node interface{ at() int }

type (
	numberLit struct {
		pos  int
		text string
	}
	stringLit struct {
		pos  int
		text string
	}
	boolLit struct {
		pos   int
		value bool
	}
	columnRef struct {
		pos  int
		name string
	}
	valueRef struct{ pos int }
	call     struct {
		pos  int
		name string
		args []node
	}
	unary struct {
		pos int
		op  string
		x   node
	}
	binary struct {
		pos  int
		op   string
		l, r node
	}
)

func (n *numberLit) at() int { return n.pos }
func (n *stringLit) at() int { return n.pos }
func (n *boolLit) at() int   { return n.pos }
func (n *columnRef) at() int { return n.pos }
func (n *valueRef) at() int  { return n.pos }
func (n *call) at() int      { return n.pos }
func (n *unary) at() int     { return n.pos }
func (n *binary) at() int    { return n.pos }

// maxDepth bounds nesting, so a pathological formula cannot exhaust the
// stack.
const maxDepth = 200

type parser struct {
	toks  []token
	i     int
	depth int
}

func parse(src string) (node, error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	if toks[0].kind == tokEOF {
		return nil, &Error{Msg: "the formula is empty"}
	}
	p := &parser{toks: toks}
	n, err := p.expr()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tokEOF {
		return nil, errorAt(t.pos, "unexpected %s", describe(t))
	}
	return n, nil
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tokEOF {
		p.i++
	}
	return t
}

func (p *parser) isOp(op string) bool {
	t := p.peek()
	return t.kind == tokOp && t.text == op
}

func (p *parser) isName(name string) bool {
	t := p.peek()
	return t.kind == tokName && t.text == name
}

func describe(t token) string {
	switch t.kind {
	case tokEOF:
		return "end of formula"
	case tokNumber:
		return "number " + t.text
	case tokString:
		return fmt.Sprintf("text %q", t.text)
	case tokColumn:
		return "column [" + t.text + "]"
	case tokName:
		return "\"" + t.text + "\""
	}
	return "\"" + t.text + "\""
}

// Precedence, lowest first: or, and, not, comparisons, &, + -, * / %,
// unary minus.
func (p *parser) expr() (node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return nil, errorAt(p.peek().pos, "the formula is nested too deeply")
	}
	return p.or()
}

func (p *parser) or() (node, error) {
	l, err := p.and()
	for err == nil && p.isName("or") {
		t := p.next()
		var r node
		if r, err = p.and(); err == nil {
			l = &binary{t.pos, "or", l, r}
		}
	}
	return l, err
}

func (p *parser) and() (node, error) {
	l, err := p.not()
	for err == nil && p.isName("and") {
		t := p.next()
		var r node
		if r, err = p.not(); err == nil {
			l = &binary{t.pos, "and", l, r}
		}
	}
	return l, err
}

func (p *parser) not() (node, error) {
	if p.isName("not") {
		t := p.next()
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > maxDepth {
			return nil, errorAt(t.pos, "the formula is nested too deeply")
		}
		x, err := p.not()
		if err != nil {
			return nil, err
		}
		return &unary{t.pos, "not", x}, nil
	}
	return p.comparison()
}

func (p *parser) comparison() (node, error) {
	l, err := p.concat()
	for err == nil {
		t := p.peek()
		if t.kind != tokOp || !strings.Contains(" = <> < <= > >= ", " "+t.text+" ") {
			break
		}
		p.next()
		var r node
		if r, err = p.concat(); err == nil {
			l = &binary{t.pos, t.text, l, r}
		}
	}
	return l, err
}

func (p *parser) concat() (node, error) {
	l, err := p.sum()
	for err == nil && p.isOp("&") {
		t := p.next()
		var r node
		if r, err = p.sum(); err == nil {
			l = &binary{t.pos, "&", l, r}
		}
	}
	return l, err
}

func (p *parser) sum() (node, error) {
	l, err := p.product()
	for err == nil && (p.isOp("+") || p.isOp("-")) {
		t := p.next()
		var r node
		if r, err = p.product(); err == nil {
			l = &binary{t.pos, t.text, l, r}
		}
	}
	return l, err
}

func (p *parser) product() (node, error) {
	l, err := p.unary()
	for err == nil && (p.isOp("*") || p.isOp("/") || p.isOp("%")) {
		t := p.next()
		var r node
		if r, err = p.unary(); err == nil {
			l = &binary{t.pos, t.text, l, r}
		}
	}
	return l, err
}

func (p *parser) unary() (node, error) {
	if p.isOp("-") || p.isOp("+") {
		t := p.next()
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > maxDepth {
			return nil, errorAt(t.pos, "the formula is nested too deeply")
		}
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		if t.text == "+" {
			return x, nil
		}
		return &unary{t.pos, "-", x}, nil
	}
	return p.primary()
}

func (p *parser) primary() (node, error) {
	t := p.next()
	switch t.kind {
	case tokNumber:
		return &numberLit{t.pos, t.text}, nil
	case tokString:
		return &stringLit{t.pos, t.text}, nil
	case tokColumn:
		return &columnRef{t.pos, t.text}, nil
	case tokName:
		switch t.text {
		case "true", "false":
			return &boolLit{t.pos, t.text == "true"}, nil
		case "value":
			return &valueRef{t.pos}, nil
		case "and", "or", "not":
			return nil, errorAt(t.pos, "unexpected %q", t.text)
		}
		if !p.isOp("(") {
			return nil, errorAt(t.pos, "unknown name %q: write columns in brackets, like [%s], and text in quotes, like \"%s\"", t.text, t.text, t.text)
		}
		p.next()
		c := &call{pos: t.pos, name: t.text}
		if p.isOp(")") {
			p.next()
			return c, nil
		}
		for {
			arg, err := p.expr()
			if err != nil {
				return nil, err
			}
			c.args = append(c.args, arg)
			if p.isOp(",") {
				p.next()
				continue
			}
			if !p.isOp(")") {
				return nil, errorAt(p.peek().pos, "expected , or ) but found %s", describe(p.peek()))
			}
			p.next()
			return c, nil
		}
	case tokOp:
		if t.text == "(" {
			n, err := p.expr()
			if err != nil {
				return nil, err
			}
			if !p.isOp(")") {
				return nil, errorAt(p.peek().pos, "expected ) but found %s", describe(p.peek()))
			}
			p.next()
			return n, nil
		}
	}
	return nil, errorAt(t.pos, "unexpected %s", describe(t))
}
