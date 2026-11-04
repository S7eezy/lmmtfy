package engine

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/csvio"
	"github.com/S7eezy/lmmtfy/internal/report"
)

type resolution struct {
	active          []*active // output columns, in output order
	order           []*active // every column, helpers included, in evaluation order
	missing         []report.MissingColumn
	missingRequired []report.MissingColumn
	aliased         []report.AliasUse
	unused          []string // source columns no rule reads: dropped
	used            []int    // indexes of the source columns rules read
}

// requiredError explains the required source columns missing from an
// input, with the column each one may have become.
func (res resolution) requiredError() error {
	parts := make([]string, len(res.missingRequired))
	for i, m := range res.missingRequired {
		parts[i] = fmt.Sprintf("%q (for %s", m.Source, m.Target)
		if m.Suggestion != "" {
			parts[i] += fmt.Sprintf("; did you mean %q?", m.Suggestion)
		}
		parts[i] += ")"
	}
	return &FileError{Reason: "required source columns missing from input: " + strings.Join(parts, ", ")}
}

// resolve matches the plan's columns against one input's header (nil
// without header) and field count.
func (p *Plan) resolve(header []string, width int) resolution {
	var res resolution
	index := map[string]int{}
	for i, h := range header {
		if _, dup := index[h]; !dup {
			index[h] = i
		}
	}
	used := make([]bool, width)
	byID := make([]*active, len(p.nodes))

	for _, c := range p.nodes {
		a := &active{column: c, pos: -1, defaultOut: c.defaultOut}
		if !c.hasSource {
			if !c.computes() {
				a.constant, a.constDefault = true, true
			}
			byID[c.id] = a
			continue
		}

		idx, found := c.index, c.index >= 0 && c.index < width
		if c.index < 0 {
			idx, found = index[c.source]
			for _, alias := range c.aliases {
				if found {
					break
				}
				if idx, found = index[alias]; found {
					res.aliased = append(res.aliased, report.AliasUse{Source: c.source, Alias: alias, Target: c.name})
				}
			}
		}
		if found {
			a.idx = idx
			used[idx] = true
			byID[c.id] = a
			continue
		}

		missing := report.MissingColumn{Source: c.sourceLabel(), Target: c.name}
		switch c.ifMissing {
		case "empty":
			if c.formula != nil {
				a.absent = true // the formula still runs, on an empty value
			} else {
				a.constant, a.constOut, a.constEmpty = true, p.nullOut, true
			}
			missing.Action = "empty"
		case "default":
			a.constant, a.constDefault = true, true
			missing.Action = "default"
		case "omit":
			missing.Action = "omitted"
			res.missing = append(res.missing, missing)
			continue
		default:
			missing.Action = "error"
			res.missingRequired = append(res.missingRequired, missing)
			continue
		}
		res.missing = append(res.missing, missing)
		byID[c.id] = a
	}

	for _, a := range byID {
		if a != nil && a.constDefault {
			a.constOut, a.constEmpty = a.fallback(p), a.fallbackEmpty()
		}
	}
	for _, c := range p.cols {
		if a := byID[c.id]; a != nil {
			a.pos = len(res.active)
			res.active = append(res.active, a)
		}
	}
	for _, id := range p.order {
		if a := byID[id]; a != nil {
			res.order = append(res.order, a)
		}
	}

	for i, u := range used {
		if u {
			res.used = append(res.used, i)
		}
		if !u {
			if header != nil {
				res.unused = append(res.unused, header[i])
			} else {
				res.unused = append(res.unused, fmt.Sprintf("#%d", i+1))
			}
		}
	}
	if header != nil {
		for _, list := range [][]report.MissingColumn{res.missing, res.missingRequired} {
			for i := range list {
				list[i].Suggestion = suggest(list[i].Source, res.unused)
			}
		}
	}
	return res
}

// ColumnCheck is how one target column will be filled from a given input.
type ColumnCheck struct {
	Target     string `json:"target"`
	Source     string `json:"source,omitempty"`     // "" when the column has no source
	Status     string `json:"status"`               // "found", "missing", "constant", "computed" (formula, cases or sequence)
	Action     string `json:"action,omitempty"`     // for missing columns: empty, default, omitted, error
	Alias      string `json:"alias,omitempty"`      // the alias it was found as, if not its source name
	Suggestion string `json:"suggestion,omitempty"` // for missing columns: the unused input column most like it
}

// InputCheck is the result of checking an input header against the plan.
type InputCheck struct {
	Columns []ColumnCheck `json:"columns"`
	Unused  []string      `json:"unused,omitempty"` // source columns that are dropped
	OK      bool          `json:"ok"`               // false when a required source column is missing
}

// CheckInput reads only the header (or first row) of an input and reports
// how each target column will be filled.
func (p *Plan) CheckInput(src io.Reader) (*InputCheck, error) {
	reader, err := csvio.NewReader(src, p.reader)
	if err != nil {
		return nil, err
	}
	rec, err := readRecord(reader)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	r := &run{plan: p}
	if p.header {
		if errors.Is(err, io.EOF) {
			return nil, &FileError{Reason: "input is empty: no header row"}
		}
		r.rawHeader = rec.Fields
	}
	header, err := r.normalizeHeader()
	if err != nil {
		return nil, err
	}

	res := p.resolve(header, len(rec.Fields))
	check := &InputCheck{Unused: res.unused, OK: len(res.missingRequired) == 0}
	missing := map[string]report.MissingColumn{}
	for _, m := range append(res.missing, res.missingRequired...) {
		missing[m.Target] = m
	}
	aliases := map[string]string{}
	for _, a := range res.aliased {
		aliases[a.Target] = a.Alias
	}
	for _, c := range p.cols {
		cc := ColumnCheck{Target: c.name}
		if c.hasSource {
			cc.Source = c.sourceLabel()
		}
		switch m, isMissing := missing[c.name]; {
		case !c.hasSource && c.computes():
			cc.Status = "computed"
		case !c.hasSource:
			cc.Status = "constant"
		case isMissing:
			cc.Status, cc.Action, cc.Suggestion = "missing", m.Action, m.Suggestion
		default:
			cc.Status, cc.Alias = "found", aliases[c.name]
		}
		check.Columns = append(check.Columns, cc)
	}
	return check, nil
}
