package engine

import (
	"errors"
	"fmt"
	"io"

	"lmmtfy/internal/csvio"
	"lmmtfy/internal/report"
)

type resolution struct {
	active          []*active
	missing         []report.MissingColumn
	missingRequired []string
	unused          []string // source columns no rule reads: dropped
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

	for _, c := range p.cols {
		a := &active{column: c}
		if !c.hasSource {
			a.constant, a.constOut = true, c.fallback(p)
			res.active = append(res.active, a)
			continue
		}

		idx, found := c.index, c.index >= 0 && c.index < width
		if c.index < 0 {
			idx, found = index[c.source]
		}
		if found {
			a.idx = idx
			used[idx] = true
			res.active = append(res.active, a)
			continue
		}

		missing := report.MissingColumn{Source: c.sourceLabel(), Target: c.name}
		switch c.ifMissing {
		case "empty":
			a.constant, a.constOut = true, p.nullOut
			missing.Action = "empty"
		case "default":
			a.constant, a.constOut = true, *c.defaultOut
			missing.Action = "default"
		case "omit":
			missing.Action = "omitted"
			res.missing = append(res.missing, missing)
			continue
		default:
			res.missingRequired = append(res.missingRequired, fmt.Sprintf("%q (for %s)", c.sourceLabel(), c.name))
			continue
		}
		res.missing = append(res.missing, missing)
		res.active = append(res.active, a)
	}

	for i, u := range used {
		if !u {
			if header != nil {
				res.unused = append(res.unused, header[i])
			} else {
				res.unused = append(res.unused, fmt.Sprintf("#%d", i+1))
			}
		}
	}
	return res
}

// ColumnCheck is how one target column will be filled from a given input.
type ColumnCheck struct {
	Target string
	Source string // "" when the column has no source
	Status string // "found", "missing", "constant"
	Action string // for missing columns: empty, default, omitted, error
}

// InputCheck is the result of checking an input header against the plan.
type InputCheck struct {
	Columns []ColumnCheck
	Unused  []string // source columns that are dropped
	OK      bool     // false when a required source column is missing
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
	for _, m := range res.missing {
		missing[m.Target] = m
	}
	for _, c := range p.cols {
		cc := ColumnCheck{Target: c.name}
		if c.hasSource {
			cc.Source = c.sourceLabel()
		}
		switch m, isMissing := missing[c.name]; {
		case !c.hasSource:
			cc.Status = "constant"
		case isMissing:
			cc.Status, cc.Action = "missing", m.Action
		case activeContains(res.active, c):
			cc.Status = "found"
		default:
			cc.Status, cc.Action = "missing", "error"
		}
		check.Columns = append(check.Columns, cc)
	}
	return check, nil
}

func activeContains(list []*active, c *column) bool {
	for _, a := range list {
		if a.column == c {
			return true
		}
	}
	return false
}
