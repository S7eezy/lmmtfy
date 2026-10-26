package config

import (
	"github.com/goccy/go-yaml"
)

// document mirrors Mapping with rules as an ordered list, so files written
// for people list rules in target column order, and the target either
// inline or as a path.
type document struct {
	Version     int           `yaml:"version"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description,omitempty"`
	Target      any           `yaml:"target"`
	Input       Input         `yaml:"input"`
	OnError     OnError       `yaml:"on_error"`
	Logging     Logging       `yaml:"logging,omitempty"`
	Columns     yaml.MapSlice `yaml:"columns"`
}

// Format serializes a mapping for people to read and edit: rules in target
// column order, the target as a path when targetPath is not empty, inline
// otherwise. Extra options (comments) are passed to the YAML encoder.
func Format(m *Mapping, targetPath string, opts ...yaml.EncodeOption) ([]byte, error) {
	doc := document{
		Version:     m.Version,
		Name:        m.Name,
		Description: m.Description,
		Target:      m.Target.Inline,
		Input:       m.Input,
		OnError:     m.OnError,
		Logging:     m.Logging,
	}
	if targetPath != "" {
		doc.Target = targetPath
	}
	if t := m.Target.Inline; t != nil {
		for _, c := range t.Columns {
			if r, ok := m.Columns[c.Name]; ok {
				doc.Columns = append(doc.Columns, yaml.MapItem{Key: c.Name, Value: r})
			}
		}
	}
	return yaml.MarshalWithOptions(doc, append(opts, yaml.UseLiteralStyleIfMultiline(true))...)
}

// FormatTarget serializes a target file.
func FormatTarget(t *Target) ([]byte, error) {
	out := *t
	if out.Version == 0 {
		out.Version = Version
	}
	return yaml.MarshalWithOptions(&out, yaml.UseLiteralStyleIfMultiline(true))
}
