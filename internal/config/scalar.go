package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// Scalar is a YAML scalar kept exactly as written.
//
// Plain YAML decoding turns `01` into 1 and `1.50` into 1.5, which is wrong
// for CSV values, codes and defaults. Scalar keeps the source text: `01` stays
// "01", `1.50` stays "1.50", `yes` stays "yes". Quoted strings are unescaped
// as usual ("\t" is a tab).
type Scalar string

// UnmarshalYAML implements yaml.NodeUnmarshaler.
func (s *Scalar) UnmarshalYAML(node ast.Node) error {
	text, err := scalarText(node)
	if err != nil {
		return err
	}
	*s = Scalar(text)
	return nil
}

// String returns the scalar text.
func (s Scalar) String() string { return string(s) }

func scalarText(node ast.Node) (string, error) {
	switch n := node.(type) {
	case *ast.NullNode:
		return "", nil
	case *ast.StringNode:
		return n.Value, nil
	case *ast.LiteralNode:
		return n.Value.Value, nil
	case *ast.TagNode:
		return scalarText(n.Value)
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		return "", fmt.Errorf("line %d: expected a single value, not a list or mapping", line(node))
	default:
		// Numbers, booleans and the like: keep the token text, not its typed value.
		return node.GetToken().Value, nil
	}
}

func line(node ast.Node) int {
	if tok := node.GetToken(); tok != nil && tok.Position != nil {
		return tok.Position.Line
	}
	return 0
}

// Pair is one entry of a ValueMap.
type Pair struct {
	From string
	To   string
}

// ValueMap is an ordered `from: to` mapping of cell values, with keys and
// values kept as written (see Scalar).
type ValueMap []Pair

// UnmarshalYAML implements yaml.NodeUnmarshaler.
func (m *ValueMap) UnmarshalYAML(node ast.Node) error {
	var entries []*ast.MappingValueNode
	switch n := node.(type) {
	case *ast.NullNode:
		*m = nil
		return nil
	case *ast.MappingNode:
		entries = n.Values
	case *ast.MappingValueNode:
		entries = []*ast.MappingValueNode{n}
	default:
		return fmt.Errorf("line %d: map must be a mapping of `source value: output value`", line(node))
	}

	seen := make(map[string]bool, len(entries))
	out := make(ValueMap, 0, len(entries))
	for _, e := range entries {
		from, err := scalarText(e.Key)
		if err != nil {
			return err
		}
		to, err := scalarText(e.Value)
		if err != nil {
			return err
		}
		if seen[from] {
			return fmt.Errorf("line %d: duplicate map key %q", line(e.Key), from)
		}
		seen[from] = true
		out = append(out, Pair{From: from, To: to})
	}
	*m = out
	return nil
}

// MarshalYAML implements yaml.InterfaceMarshaler, preserving entry order.
func (m ValueMap) MarshalYAML() (any, error) {
	out := make(yaml.MapSlice, len(m))
	for i, p := range m {
		out[i] = yaml.MapItem{Key: p.From, Value: p.To}
	}
	return out, nil
}
