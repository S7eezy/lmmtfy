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
	Version     int            `yaml:"version,omitempty" json:"version,omitempty"`
	Name        string         `yaml:"name,omitempty" json:"name,omitempty"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Output      Output         `yaml:"output" json:"output"`
	Columns     []TargetColumn `yaml:"columns" json:"columns"`
}

// Output is the CSV dialect and encoding of produced files.
type Output struct {
	Encoding   string `yaml:"encoding,omitempty" json:"encoding,omitempty"`
	BOM        bool   `yaml:"bom,omitempty" json:"bom,omitempty"`
	Delimiter  Scalar `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
	Quote      Scalar `yaml:"quote,omitempty" json:"quote,omitempty"`
	QuoteStyle string `yaml:"quote_style,omitempty" json:"quote_style,omitempty"`
	Escape     string `yaml:"escape,omitempty" json:"escape,omitempty"`
	LineEnding string `yaml:"line_ending,omitempty" json:"line_ending,omitempty"`
	Header     *bool  `yaml:"header,omitempty" json:"header,omitempty"`
	NullValue  Scalar `yaml:"null_value,omitempty" json:"null_value,omitempty"`
}

// TargetColumn is one output column. Type options describe the output
// representation; which options are allowed depends on Type.
type TargetColumn struct {
	Name     string  `yaml:"name" json:"name"`
	Type     string  `yaml:"type" json:"type"`
	Required bool    `yaml:"required,omitempty" json:"required,omitempty"`
	Default  *Scalar `yaml:"default,omitempty" json:"default,omitempty"`

	// string
	MaxLength     *int     `yaml:"max_length,omitempty" json:"max_length,omitempty"`
	OnOverflow    string   `yaml:"on_overflow,omitempty" json:"on_overflow,omitempty"`
	Case          string   `yaml:"case,omitempty" json:"case,omitempty"`
	Pattern       string   `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	AllowedValues []Scalar `yaml:"allowed_values,omitempty" json:"allowed_values,omitempty"`

	// integer, decimal
	Min          *Scalar `yaml:"min,omitempty" json:"min,omitempty"`
	Max          *Scalar `yaml:"max,omitempty" json:"max,omitempty"`
	Scale        *int    `yaml:"scale,omitempty" json:"scale,omitempty"`
	Rounding     string  `yaml:"rounding,omitempty" json:"rounding,omitempty"`
	DecimalSep   Scalar  `yaml:"decimal_sep,omitempty" json:"decimal_sep,omitempty"`
	ThousandsSep Scalar  `yaml:"thousands_sep,omitempty" json:"thousands_sep,omitempty"`

	// boolean
	TrueValue  *Scalar `yaml:"true_value,omitempty" json:"true_value,omitempty"`
	FalseValue *Scalar `yaml:"false_value,omitempty" json:"false_value,omitempty"`

	// date, datetime, time
	Format   string `yaml:"format,omitempty" json:"format,omitempty"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}

// Mapping describes how one source format becomes a target.
type Mapping struct {
	Version     int             `yaml:"version" json:"version"`
	Name        string          `yaml:"name" json:"name"`
	Description string          `yaml:"description,omitempty" json:"description,omitempty"`
	Target      TargetRef       `yaml:"target" json:"target"`
	Input       Input           `yaml:"input" json:"input"`
	OnError     OnError         `yaml:"on_error" json:"on_error"`
	Logging     Logging         `yaml:"logging" json:"logging"`
	Columns     map[string]Rule `yaml:"columns" json:"columns"`
}

// TargetRef is either a path to a target file or an inline target.
// After Load, Inline is always set (the file, if any, has been read).
type TargetRef struct {
	Path   string  `json:"path,omitempty"`
	Inline *Target `json:"inline,omitempty"`
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
	Encoding            string   `yaml:"encoding,omitempty" json:"encoding,omitempty"`
	BOM                 Scalar   `yaml:"bom,omitempty" json:"bom,omitempty"`
	Delimiter           Scalar   `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
	Quote               Scalar   `yaml:"quote,omitempty" json:"quote,omitempty"`
	Escape              string   `yaml:"escape,omitempty" json:"escape,omitempty"`
	Header              *bool    `yaml:"header,omitempty" json:"header,omitempty"`
	HeaderRow           int      `yaml:"header_row,omitempty" json:"header_row,omitempty"`
	SkipRowsAfterHeader int      `yaml:"skip_rows_after_header,omitempty" json:"skip_rows_after_header,omitempty"`
	SkipFooter          int      `yaml:"skip_footer,omitempty" json:"skip_footer,omitempty"`
	CommentPrefix       Scalar   `yaml:"comment_prefix,omitempty" json:"comment_prefix,omitempty"`
	NullTokens          []Scalar `yaml:"null_tokens,omitempty" json:"null_tokens,omitempty"`
	Trim                bool     `yaml:"trim,omitempty" json:"trim,omitempty"`
	RaggedRows          string   `yaml:"ragged_rows,omitempty" json:"ragged_rows,omitempty"`
	DuplicateHeaders    string   `yaml:"duplicate_headers,omitempty" json:"duplicate_headers,omitempty"`
}

// OnError is the global error strategy.
type OnError struct {
	Strategy     string   `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	MaxErrorRate *float64 `yaml:"max_error_rate,omitempty" json:"max_error_rate,omitempty"`
	MaxErrors    *int     `yaml:"max_errors,omitempty" json:"max_errors,omitempty"`
	RejectFile   *bool    `yaml:"reject_file,omitempty" json:"reject_file,omitempty"`
}

// Logging configures conversion logs.
type Logging struct {
	Level   string `yaml:"level,omitempty" json:"level,omitempty"`
	Format  string `yaml:"format,omitempty" json:"format,omitempty"`
	File    string `yaml:"file,omitempty" json:"file,omitempty"`
	Summary *bool  `yaml:"summary,omitempty" json:"summary,omitempty"`
}

// Rule says how one target column is filled. Parse options describe the
// source representation.
type Rule struct {
	Source       *Scalar  `yaml:"source,omitempty" json:"source,omitempty"`
	SourceIndex  int      `yaml:"source_index,omitempty" json:"source_index,omitempty"`
	Optional     bool     `yaml:"optional,omitempty" json:"optional,omitempty"`
	IfMissing    string   `yaml:"if_missing,omitempty" json:"if_missing,omitempty"`
	IfEmpty      string   `yaml:"if_empty,omitempty" json:"if_empty,omitempty"`
	Default      *Scalar  `yaml:"default,omitempty" json:"default,omitempty"`
	OnError      string   `yaml:"on_error,omitempty" json:"on_error,omitempty"`
	Map          ValueMap `yaml:"map,omitempty" json:"map,omitempty"`
	MapUnmatched string   `yaml:"map_unmatched,omitempty" json:"map_unmatched,omitempty"`
	Trim         *bool    `yaml:"trim,omitempty" json:"trim,omitempty"`

	Formats      []string `yaml:"formats,omitempty" json:"formats,omitempty"`
	DecimalSep   Scalar   `yaml:"decimal_sep,omitempty" json:"decimal_sep,omitempty"`
	ThousandsSep Scalar   `yaml:"thousands_sep,omitempty" json:"thousands_sep,omitempty"`
	TrueValues   []Scalar `yaml:"true_values,omitempty" json:"true_values,omitempty"`
	FalseValues  []Scalar `yaml:"false_values,omitempty" json:"false_values,omitempty"`
	Timezone     string   `yaml:"timezone,omitempty" json:"timezone,omitempty"`
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
