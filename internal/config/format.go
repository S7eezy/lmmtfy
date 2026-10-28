package config

import (
	"maps"
	"slices"

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
	Group       *Group        `yaml:"group,omitempty"`
	Helpers     yaml.MapSlice `yaml:"helpers,omitempty"`
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
		Group:       m.Group,
	}
	if targetPath != "" {
		doc.Target = targetPath
	}
	for _, name := range slices.Sorted(maps.Keys(m.Helpers)) {
		doc.Helpers = append(doc.Helpers, yaml.MapItem{Key: name, Value: m.Helpers[name]})
	}
	// Patterns (price_{n}) come where their first column is.
	slots, _ := m.Slots()
	written := map[string]bool{}
	if t := m.Target.Inline; t != nil {
		for _, c := range t.Columns {
			key := c.Name
			if s, ok := slots.Of[c.Name]; ok {
				key = s.Pattern
			}
			if r, ok := m.Columns[key]; ok && !written[key] {
				doc.Columns = append(doc.Columns, yaml.MapItem{Key: key, Value: r})
				written[key] = true
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(m.Columns)) {
		if IsPattern(key) && !written[key] { // matching nothing: kept, not lost
			doc.Columns = append(doc.Columns, yaml.MapItem{Key: key, Value: m.Columns[key]})
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
