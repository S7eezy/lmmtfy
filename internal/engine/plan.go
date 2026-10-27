package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/types"
)

// Plan is a compiled mapping, ready to convert any number of files.
type Plan struct {
	Mapping *config.Mapping
	Hash    string

	reader  csvio.ReaderOptions
	writer  csvio.WriterOptions
	rejects csvio.WriterOptions

	header          bool
	skipAfterHeader int
	ragged          string
	dupHeaders      string
	nulls           map[string]bool
	nullOut         string
	outHeader       bool

	strategy  string
	maxRate   *float64
	maxErrors *int

	cols []*column
}

// column is the compiled rule of one target column.
type column struct {
	name      string
	required  bool
	numeric   bool
	hasSource bool
	source    string // header name
	index     int    // 0-based position when the rule uses source_index, else -1

	ifMissing string
	ifEmpty   string
	unmatched string
	onError   string
	trim      bool
	vmap      map[string]string

	conv       types.Converter
	defaultOut *string // default in its final output form
}

// sourceLabel names the source column in messages.
func (c *column) sourceLabel() string {
	if c.index >= 0 {
		return fmt.Sprintf("#%d", c.index+1)
	}
	return c.source
}

// Compile validates a mapping (defaults applied, target resolved) and builds
// its plan. The plan is nil when there is at least one error; warnings are
// returned either way.
func Compile(m *config.Mapping) (*Plan, config.Issues) {
	issues := config.Validate(m)
	t := m.Target.Inline
	if t == nil {
		return nil, issues
	}
	o := t.Output
	in := m.Input

	if _, err := csvio.LookupEncoding(in.Encoding); err != nil {
		issues.Errorf("input.encoding", "%v", err)
	}
	if _, err := csvio.LookupEncoding(o.Encoding); err != nil {
		issues.Errorf("target.output.encoding", "%v", err)
	}

	p := &Plan{
		Mapping: m,
		reader: csvio.ReaderOptions{
			Dialect:       csvio.Dialect{Delimiter: config.Char(in.Delimiter), Quote: config.Char(in.Quote), Escape: csvio.Escape(in.Escape)},
			Encoding:      in.Encoding,
			BOM:           string(in.BOM),
			SkipLines:     in.HeaderRow - 1,
			CommentPrefix: string(in.CommentPrefix),
			SkipFooter:    in.SkipFooter,
		},
		header:          *in.Header,
		skipAfterHeader: in.SkipRowsAfterHeader,
		ragged:          in.RaggedRows,
		dupHeaders:      in.DuplicateHeaders,
		nulls:           map[string]bool{},
		nullOut:         string(o.NullValue),
		outHeader:       *o.Header,
		strategy:        m.OnError.Strategy,
		maxRate:         m.OnError.MaxErrorRate,
		maxErrors:       m.OnError.MaxErrors,
	}
	for _, tok := range in.NullTokens {
		p.nulls[string(tok)] = true
	}

	lineEnding := "\r\n"
	if o.LineEnding == "lf" {
		lineEnding = "\n"
	}
	p.writer = csvio.WriterOptions{
		Dialect:    csvio.Dialect{Delimiter: config.Char(o.Delimiter), Quote: config.Char(o.Quote), Escape: csvio.Escape(o.Escape)},
		Encoding:   o.Encoding,
		BOM:        o.BOM,
		QuoteStyle: csvio.QuoteStyle(o.QuoteStyle),
		LineEnding: lineEnding,
	}
	// Reject files use the input dialect and encoding, so they can be fixed
	// and fed back through the same mapping.
	p.rejects = csvio.WriterOptions{
		Dialect:    p.reader.Dialect,
		Encoding:   in.Encoding,
		QuoteStyle: csvio.QuoteMinimal,
		LineEnding: lineEnding,
	}
	if p.rejects.Escape == csvio.EscapeNone {
		p.rejects.Escape = csvio.EscapeDouble
	}

	for _, tc := range t.Columns {
		if c := compileColumn(&issues, m, tc); c != nil {
			p.cols = append(p.cols, c)
		}
	}

	if issues.HasErrors() {
		return nil, issues
	}
	hash, err := config.Hash(m)
	if err != nil {
		issues.Errorf("", "cannot serialize mapping: %v", err)
		return nil, issues
	}
	p.Hash = hash
	return p, issues
}

func compileColumn(issues *config.Issues, m *config.Mapping, tc config.TargetColumn) *column {
	rule, mapped := m.Columns[tc.Name]
	c := &column{
		name:      tc.Name,
		required:  tc.Required,
		numeric:   tc.Type == "integer" || tc.Type == "decimal",
		hasSource: rule.HasSource(),
		index:     rule.SourceIndex - 1,
		ifMissing: rule.IfMissing,
		ifEmpty:   rule.IfEmpty,
		unmatched: rule.MapUnmatched,
		onError:   rule.EffectiveOnError(m),
		trim:      m.Input.Trim,
	}
	if !mapped {
		c.ifEmpty = "keep"
	}
	if rule.Source != nil {
		c.source = strings.TrimSpace(string(*rule.Source))
	}
	if rule.Trim != nil {
		c.trim = *rule.Trim
	}
	if rule.Map != nil {
		c.vmap = make(map[string]string, len(rule.Map))
		for _, pair := range rule.Map {
			c.vmap[pair.From] = pair.To
		}
	}

	if !slices.Contains(config.Types, tc.Type) {
		return nil // already reported by config.Validate
	}

	conv, err := types.New(tc, &rule)
	addTypeErrors(issues, tc.Name, err)
	c.conv = conv

	def, defPath := rule.Default, "columns."+tc.Name+".default"
	if def == nil {
		def, defPath = tc.Default, "target.columns."+tc.Name+".default"
	}
	if def != nil {
		out, err := defaultValue(tc, string(*def))
		if err != nil {
			issues.Errorf(defPath, "%q is not a valid %s for this column: %v", *def, tc.Type, err)
		}
		c.defaultOut = &out
	}
	return c
}

// defaultValue converts a default, written in the output representation,
// into its final form. An empty default means an empty value.
func defaultValue(tc config.TargetColumn, def string) (string, error) {
	if def == "" {
		return "", nil
	}
	conv, err := types.New(tc, nil)
	if err != nil {
		return "", err
	}
	out, err := conv.Convert(def)
	if err != nil {
		return "", err // a truncated default is an error too: fix the config
	}
	return out, nil
}

func addTypeErrors(issues *config.Issues, column string, err error) {
	if err == nil {
		return
	}
	var list []error
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		list = j.Unwrap()
	} else {
		list = []error{err}
	}
	for _, e := range list {
		var oe *types.OptionError
		if !errors.As(e, &oe) {
			issues.Errorf("columns."+column, "%v", e)
			continue
		}
		option := strings.TrimPrefix(oe.Option, "target:")
		if oe.IsTarget() {
			issues.Errorf("target.columns."+column+"."+option, "%s", oe.Msg)
		} else {
			issues.Errorf("columns."+column+"."+option, "%s", oe.Msg)
		}
	}
}
