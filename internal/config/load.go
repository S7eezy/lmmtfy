package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// LoadMapping reads a mapping file, loads its target file if it references
// one (relative to the mapping's directory), and applies defaults.
func LoadMapping(path string) (*Mapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := ParseMapping(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if m.Target.Inline == nil {
		if m.Target.Path == "" {
			return nil, fmt.Errorf("%s: target: required (a target file path or an inline target)", path)
		}
		targetPath := m.Target.Path
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(filepath.Dir(path), targetPath)
		}
		t, err := LoadTarget(targetPath)
		if err != nil {
			return nil, err
		}
		m.Target.Inline = t
	}

	m.ApplyDefaults()
	return m, nil
}

// ParseMapping decodes a mapping document without resolving its target file
// or applying defaults. Unknown fields are errors: a typo must not be
// silently ignored.
func ParseMapping(data []byte) (*Mapping, error) {
	var m Mapping
	if err := decode(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadTarget reads a target file.
func LoadTarget(path string) (*Target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Target
	if err := decode(data, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &t, nil
}

func decode(data []byte, v any) (err error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("file is empty")
	}
	// The YAML library panics on some malformed documents (found by
	// fuzzing, e.g. a tag without value): report them as invalid instead
	// of crashing.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid YAML document (%v)", r)
		}
	}()
	if err := yaml.UnmarshalWithOptions(data, v, yaml.DisallowUnknownField()); err != nil {
		// FormatError adds the offending source line with a caret under it.
		return errors.New(yaml.FormatError(err, false, true))
	}
	return nil
}

// Marshal serializes a mapping, its target inlined.
func Marshal(m *Mapping) ([]byte, error) {
	return yaml.MarshalWithOptions(m, yaml.UseLiteralStyleIfMultiline(true))
}

// Hash is the SHA-256 of the serialized resolved mapping. It identifies
// exactly which configuration produced a file.
func Hash(m *Mapping) (string, error) {
	data, err := Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ApplyDefaults fills every unset option with its default value, so the
// rest of the program never has to know what "unset" means.
func (m *Mapping) ApplyDefaults() {
	if t := m.Target.Inline; t != nil {
		o := &t.Output
		setDefault(&o.Encoding, "utf-8")
		setDefault(&o.Delimiter, ",")
		setDefault(&o.Quote, `"`)
		setDefault(&o.QuoteStyle, QuoteStyles[0])
		setDefault(&o.Escape, Escapes[0])
		setDefault(&o.LineEnding, LineEndings[0])
		if o.Header == nil {
			o.Header = ptr(true)
		}
	}

	in := &m.Input
	setDefault(&in.Encoding, "utf-8")
	setDefault(&in.BOM, Scalar(BOMModes[0]))
	setDefault(&in.Delimiter, ",")
	setDefault(&in.Quote, `"`)
	setDefault(&in.Escape, Escapes[0])
	if in.Header == nil {
		in.Header = ptr(true)
	}
	if in.HeaderRow == 0 {
		in.HeaderRow = 1
	}
	setDefault(&in.RaggedRows, RaggedRows[0])
	setDefault(&in.DuplicateHeaders, DuplicateHeaders[0])

	setDefault(&m.OnError.Strategy, Strategies[0])
	if m.OnError.RejectFile == nil {
		m.OnError.RejectFile = ptr(true)
	}

	setDefault(&m.Logging.Level, LogLevels[0])
	setDefault(&m.Logging.Format, LogFormats[0])
	if m.Logging.Summary == nil {
		m.Logging.Summary = ptr(true)
	}

	for name, r := range m.Columns {
		if r.Optional {
			setDefault(&r.IfMissing, "empty")
		} else {
			setDefault(&r.IfMissing, IfMissingValues[0])
		}
		setDefault(&r.IfEmpty, IfEmptyValues[0])
		setDefault(&r.MapUnmatched, MapUnmatched[0])
		// Rule is stored by value in the map: write the copy back.
		m.Columns[name] = r
	}
}

func setDefault[T ~string](field *T, value T) {
	if *field == "" {
		*field = value
	}
}

func ptr[T any](v T) *T { return &v }
