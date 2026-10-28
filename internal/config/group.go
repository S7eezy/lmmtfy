package config

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Group combines several input rows (a batch: rows sharing the values of
// By) into one output row. See DESIGN.md, Combining rows.
type Group struct {
	By       []string `yaml:"by" json:"by"`                                 // source columns of the batch key
	OrderBy  []string `yaml:"order_by,omitempty" json:"order_by,omitempty"` // helpers ordering a batch's rows
	Sorted   *bool    `yaml:"sorted,omitempty" json:"sorted,omitempty"`     // rows of a batch are next to each other; default true
	Overflow string   `yaml:"overflow,omitempty" json:"overflow,omitempty"` // more rows than slots: error | drop_extra
}

// GroupOverflows are the values of group.overflow, default first.
var GroupOverflows = []string{"error", "drop_extra"}

// SlotMark is the placeholder of the slot number in a rule key: price_{n}.
const SlotMark = "{n}"

// IsPattern reports whether a rule key is a slot pattern.
func IsPattern(key string) bool { return strings.Contains(key, SlotMark) }

// Slots tells which target columns the slot patterns of a mapping fill.
type Slots struct {
	// Numbers are the slot numbers found in the target, ascending: the
	// first row of a batch fills the column numbered Numbers[0].
	Numbers []int
	// Of maps a target column to its pattern and slot index.
	Of map[string]Slot
}

// Slot is the place of a target column in a pattern.
type Slot struct {
	Pattern string
	Index   int
}

// Slots matches the mapping's patterns against its target columns.
// Problems are returned as issues: a pattern matching nothing, patterns
// matching different numbers, a column matched twice.
func (m *Mapping) Slots() (Slots, Issues) {
	var is Issues
	s := Slots{Of: map[string]Slot{}}
	t := m.Target.Inline
	if t == nil {
		return s, is
	}
	var first string
	found := map[string]map[string]int{} // pattern → column → number
	for _, key := range slices.Sorted(maps.Keys(m.Columns)) {
		if !IsPattern(key) {
			continue
		}
		path := "columns." + key
		if strings.Count(key, SlotMark) != 1 {
			is.Errorf(path, "a pattern has one %s, where the slot number is", SlotMark)
			continue
		}
		before, after, _ := strings.Cut(key, SlotMark)
		re := regexp.MustCompile("^" + regexp.QuoteMeta(before) + `(\d+)` + regexp.QuoteMeta(after) + "$")
		cols := map[string]int{}
		for _, c := range t.Columns {
			if g := re.FindStringSubmatch(c.Name); g != nil {
				n, err := strconv.Atoi(g[1])
				if err == nil {
					cols[c.Name] = n
				}
			}
		}
		if len(cols) == 0 {
			is.Errorf(path, "no target column matches this pattern (%s stands for a number)", SlotMark)
			continue
		}
		numbers := slices.Sorted(maps.Values(cols))
		if first == "" {
			first, s.Numbers = key, numbers
		} else if !slices.Equal(numbers, s.Numbers) {
			is.Errorf(path, "matches the numbers %s, but %s matches %s: every pattern must cover the same numbers", span(numbers), first, span(s.Numbers))
			continue
		}
		found[key] = cols
	}
	for _, key := range slices.Sorted(maps.Keys(found)) {
		for name, n := range found[key] {
			if prev, dup := s.Of[name]; dup {
				is.Errorf("columns."+key, "%s is also matched by %s", name, prev.Pattern)
				continue
			}
			if _, plain := m.Columns[name]; plain {
				is.Errorf("columns."+name, "is also matched by the pattern %s: keep one rule", key)
				continue
			}
			s.Of[name] = Slot{Pattern: key, Index: slices.Index(s.Numbers, n)}
		}
	}
	return s, is
}

func span(numbers []int) string {
	if len(numbers) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d to %d (%d)", numbers[0], numbers[len(numbers)-1], len(numbers))
}

// RuleOf is the rule filling a target column: its own, or its pattern's.
func (m *Mapping) RuleOf(name string, slots Slots) (Rule, bool) {
	if r, ok := m.Columns[name]; ok {
		return r, true
	}
	if s, ok := slots.Of[name]; ok {
		return m.Columns[s.Pattern], true
	}
	return Rule{}, false
}

func validateGroup(is *Issues, m *Mapping, slots Slots) {
	g := m.Group
	if g == nil {
		for key := range m.Columns {
			if IsPattern(key) {
				is.Errorf("columns."+key, "patterns fill one column per row of a batch: they need group (combining rows)")
			}
		}
		return
	}
	if len(g.By) == 0 {
		is.Errorf("group.by", "at least one source column is required")
	}
	if !*m.Input.Header {
		is.Errorf("group.by", "names source columns: the input needs a header")
	}
	for i, h := range g.OrderBy {
		if _, ok := m.Helpers[h]; !ok {
			is.Errorf(fmt.Sprintf("group.order_by[%d]", i), "%q is not a helper: rows are ordered by helpers, which have a type", h)
		}
	}
	checkEnum(is, "group.overflow", g.Overflow, GroupOverflows)

	t := m.Target.Inline
	types := map[string]string{}
	for _, c := range t.Columns {
		if s, ok := slots.Of[c.Name]; ok {
			if prev, seen := types[s.Pattern]; seen && prev != c.Type {
				is.Errorf("columns."+s.Pattern, "matches columns of different types (%s, %s): slots of a pattern share one type", prev, c.Type)
			}
			types[s.Pattern] = c.Type
		}
	}

	// Outside slots, a plain source reads the batch's first row: right for
	// the key only.
	for name, r := range m.Columns {
		if IsPattern(name) || r.Source == nil {
			continue
		}
		if src := strings.TrimSpace(string(*r.Source)); !slices.Contains(g.By, src) {
			is.Errorf("columns."+name+".source", "a batch has several rows: only key columns (group.by) read a source here; use a helper and first(…), lowest(…) or a pattern column")
		}
	}
	for name, r := range m.Columns {
		if r.SourceIndex > 0 && !IsPattern(name) {
			is.Errorf("columns."+name+".source_index", "a batch has several rows: only key columns (group.by) read a source here")
		}
	}
}
