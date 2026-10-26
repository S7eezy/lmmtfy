package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// Version is the configuration schema version this engine reads and writes.
const Version = 1

// Target is the format the downstream software expects.
type Target struct {
	Version     int            `yaml:"version,omitempty"`
	Name        string         `yaml:"name,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Output      Output         `yaml:"output"`
	Columns     []TargetColumn `yaml:"columns"`
}

// Output is the CSV dialect and encoding of produced files.
type Output struct {
	Encoding   string `yaml:"encoding,omitempty"`
	BOM        bool   `yaml:"bom,omitempty"`
	Delimiter  Scalar `yaml:"delimiter,omitempty"`
	Quote      Scalar `yaml:"quote,omitempty"`
	QuoteStyle string `yaml:"quote_style,omitempty"`
	Escape     string `yaml:"escape,omitempty"`
	LineEnding string `yaml:"line_ending,omitempty"`
	Header     *bool  `yaml:"header,omitempty"`
	NullValue  Scalar `yaml:"null_value,omitempty"`
}

// TargetColumn is one output column. Type options describe the output
// representation; which options are allowed depends on Type.
type TargetColumn struct {
	Name     string  `yaml:"name"`
	Type     string  `yaml:"type"`
	Required bool    `yaml:"required,omitempty"`
	Default  *Scalar `yaml:"default,omitempty"`

	// string
	MaxLength     *int     `yaml:"max_length,omitempty"`
	OnOverflow    string   `yaml:"on_overflow,omitempty"`
	Case          string   `yaml:"case,omitempty"`
	Pattern       string   `yaml:"pattern,omitempty"`
	AllowedValues []Scalar `yaml:"allowed_values,omitempty"`

	// integer, decimal
	Min          *Scalar `yaml:"min,omitempty"`
	Max          *Scalar `yaml:"max,omitempty"`
	Scale        *int    `yaml:"scale,omitempty"`
	Rounding     string  `yaml:"rounding,omitempty"`
	DecimalSep   Scalar  `yaml:"decimal_sep,omitempty"`
	ThousandsSep Scalar  `yaml:"thousands_sep,omitempty"`

	// boolean
	TrueValue  *Scalar `yaml:"true_value,omitempty"`
	FalseValue *Scalar `yaml:"false_value,omitempty"`

	// date, datetime, time
	Format   string `yaml:"format,omitempty"`
	Timezone string `yaml:"timezone,omitempty"`
}

// Mapping describes how one source format becomes a target.
type Mapping struct {
	Version     int             `yaml:"version"`
	Name        string          `yaml:"name"`
	Description string          `yaml:"description,omitempty"`
	Target      TargetRef       `yaml:"target"`
	Input       Input           `yaml:"input"`
	OnError     OnError         `yaml:"on_error"`
	Logging     Logging         `yaml:"logging"`
	Columns     map[string]Rule `yaml:"columns"`
}

// TargetRef is either a path to a target file or an inline target.
// After Load, Inline is always set (the file, if any, has been read).
type TargetRef struct {
	Path   string
	Inline *Target
}

// UnmarshalYAML implements yaml.NodeUnmarshaler.
func (r *TargetRef) UnmarshalYAML(node ast.Node) error {
	switch node.(type) {
	case *ast.StringNode:
		path, err := scalarText(node)
		if err != nil {
			return err
		}
		r.Path = path
		return nil
	case *ast.MappingNode, *ast.MappingValueNode:
		var t Target
		if err := yaml.NodeToValue(node, &t, yaml.DisallowUnknownField()); err != nil {
			return err
		}
		r.Inline = &t
		return nil
	default:
		return fmt.Errorf("line %d: target must be a file path or an inline target", line(node))
	}
}

// MarshalYAML implements yaml.InterfaceMarshaler: the inline target when
// present (resolved mappings are self-contained), the path otherwise.
func (r TargetRef) MarshalYAML() (any, error) {
	if r.Inline != nil {
		return r.Inline, nil
	}
	return r.Path, nil
}

// Input is the dialect and layout of source files.
type Input struct {
	Encoding            string   `yaml:"encoding,omitempty"`
	BOM                 Scalar   `yaml:"bom,omitempty"`
	Delimiter           Scalar   `yaml:"delimiter,omitempty"`
	Quote               Scalar   `yaml:"quote,omitempty"`
	Escape              string   `yaml:"escape,omitempty"`
	Header              *bool    `yaml:"header,omitempty"`
	HeaderRow           int      `yaml:"header_row,omitempty"`
	SkipRowsAfterHeader int      `yaml:"skip_rows_after_header,omitempty"`
	SkipFooter          int      `yaml:"skip_footer,omitempty"`
	CommentPrefix       Scalar   `yaml:"comment_prefix,omitempty"`
	NullTokens          []Scalar `yaml:"null_tokens,omitempty"`
	Trim                bool     `yaml:"trim,omitempty"`
	RaggedRows          string   `yaml:"ragged_rows,omitempty"`
	DuplicateHeaders    string   `yaml:"duplicate_headers,omitempty"`
}

// OnError is the global error strategy.
type OnError struct {
	Strategy     string   `yaml:"strategy,omitempty"`
	MaxErrorRate *float64 `yaml:"max_error_rate,omitempty"`
	MaxErrors    *int     `yaml:"max_errors,omitempty"`
	RejectFile   *bool    `yaml:"reject_file,omitempty"`
}

// Logging configures conversion logs.
type Logging struct {
	Level   string `yaml:"level,omitempty"`
	Format  string `yaml:"format,omitempty"`
	File    string `yaml:"file,omitempty"`
	Summary *bool  `yaml:"summary,omitempty"`
}

// Rule says how one target column is filled. Parse options describe the
// source representation.
type Rule struct {
	Source       *Scalar  `yaml:"source,omitempty"`
	SourceIndex  int      `yaml:"source_index,omitempty"`
	Optional     bool     `yaml:"optional,omitempty"`
	IfMissing    string   `yaml:"if_missing,omitempty"`
	IfEmpty      string   `yaml:"if_empty,omitempty"`
	Default      *Scalar  `yaml:"default,omitempty"`
	OnError      string   `yaml:"on_error,omitempty"`
	Map          ValueMap `yaml:"map,omitempty"`
	MapUnmatched string   `yaml:"map_unmatched,omitempty"`
	Trim         *bool    `yaml:"trim,omitempty"`

	Formats      []string `yaml:"formats,omitempty"`
	DecimalSep   Scalar   `yaml:"decimal_sep,omitempty"`
	ThousandsSep Scalar   `yaml:"thousands_sep,omitempty"`
	TrueValues   []Scalar `yaml:"true_values,omitempty"`
	FalseValues  []Scalar `yaml:"false_values,omitempty"`
	Timezone     string   `yaml:"timezone,omitempty"`
}

// HasSource reports whether the rule reads a source column.
func (r Rule) HasSource() bool { return r.Source != nil || r.SourceIndex > 0 }

// Allowed values of enumerated options. The first one is the default.
var (
	Types            = []string{"string", "integer", "decimal", "boolean", "date", "datetime", "time"}
	Strategies       = []string{"fail_file", "skip_row", "default"}
	IfMissingValues  = []string{"error", "empty", "default", "omit"}
	IfEmptyValues    = []string{"keep", "default", "error"}
	MapUnmatched     = []string{"keep", "error", "default"}
	RaggedRows       = []string{"error", "pad", "truncate", "lenient"}
	DuplicateHeaders = []string{"error", "suffix"}
	Escapes          = []string{"double", "backslash", "none"}
	QuoteStyles      = []string{"minimal", "always", "never", "non_numeric"}
	LineEndings      = []string{"crlf", "lf"}
	BOMModes         = []string{"auto", "true", "false"}
	LogLevels        = []string{"errors", "none", "warnings", "info", "trace"}
	LogFormats       = []string{"text", "json"}
	Cases            = []string{"none", "upper", "lower"}
	Overflows        = []string{"error", "truncate"}
	Roundings        = []string{"half_up", "half_even", "truncate"}
)
