package types

import (
	"fmt"
	"strings"
	"time"

	// Embedded time zone database: production machines (Windows especially)
	// may have none, and lmmtfy must never depend on the host for it.
	_ "time/tzdata"
)

var defaultFormats = map[string]string{
	"date":     "YYYY-MM-DD",
	"datetime": "YYYY-MM-DDTHH:mm:ss",
	"time":     "HH:mm:ss",
}

type temporalConv struct {
	kind          string
	formats       []string // as written, for error messages
	layouts       []string
	out           string
	inLoc, outLoc *time.Location
}

func newTemporal(o *opts) Converter {
	o.use("target:format", "formats")
	c := &temporalConv{kind: o.col.Type, inLoc: time.UTC}

	outFormat := o.col.Format
	if outFormat == "" {
		outFormat = defaultFormats[c.kind]
	}
	out, err := Layout(outFormat)
	if err != nil {
		o.errorf("target:format", "%v", err)
	}
	c.out = out

	c.formats = o.rule.Formats
	if c.formats == nil || o.fromOutput {
		c.formats = []string{outFormat}
	}
	if len(c.formats) == 0 {
		o.errorf("formats", "at least one format is required")
	}
	for _, f := range c.formats {
		layout, err := Layout(f)
		if err != nil {
			o.errorf("formats", "%v", err)
		}
		c.layouts = append(c.layouts, layout)
	}

	// Time zones only make sense for datetimes: dates and times of day are
	// written as they are read.
	if c.kind == "datetime" {
		o.use("target:timezone", "timezone")
		c.outLoc = loadLocation(o, "target:timezone", o.col.Timezone)
		c.inLoc = c.outLoc
		if o.rule.Timezone != "" && !o.fromOutput {
			c.inLoc = loadLocation(o, "timezone", o.rule.Timezone)
		}
	}
	return c
}

func loadLocation(o *opts, option, name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		o.errorf(option, "unknown time zone %q (use names such as UTC or Europe/Paris)", name)
		return time.UTC
	}
	return loc
}

// Convert tries each input format in order. Values carrying their own offset
// (Z or ZZ tokens) keep it; others are read in the input time zone.
func (c *temporalConv) Convert(raw string) (string, error) {
	for _, layout := range c.layouts {
		t, err := time.ParseInLocation(layout, raw, c.inLoc)
		if err != nil {
			continue
		}
		if c.outLoc != nil {
			t = t.In(c.outLoc)
		}
		return t.Format(c.out), nil
	}
	if len(c.formats) == 1 {
		return "", fmt.Errorf("%q does not match %s format %s", shorten(raw), c.kind, c.formats[0])
	}
	return "", fmt.Errorf("%q does not match any %s format (%s)", shorten(raw), c.kind, strings.Join(c.formats, ", "))
}
