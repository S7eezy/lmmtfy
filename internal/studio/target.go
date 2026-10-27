package studio

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/detect"
	"github.com/S7eezy/lmmtfy/internal/engine"
)

// TargetDraft is a target drafted from an example of the expected file.
type TargetDraft struct {
	Target    *config.Target `json:"target"`
	Detection *detect.Result `json:"detection"`
}

// DraftTarget proposes a target from an example of the file the receiving
// software expects. Unlike a mapping's draft, which normalizes the output
// (ISO dates, "." decimals), it keeps what the example shows, since that is
// the format to produce: header names as written, delimiter, encoding,
// line endings, date formats, separators. The columns of the result are in
// the order of Detection.Columns.
func DraftTarget(path string) (*TargetDraft, error) {
	res, err := detect.File(path, detect.Options{})
	if err != nil {
		return nil, err
	}
	header := res.Header
	t := &config.Target{
		Version: config.Version,
		Output: config.Output{
			Encoding:   res.Encoding,
			BOM:        res.BOM,
			Delimiter:  config.Scalar(res.Delimiter),
			Quote:      config.Scalar(res.Quote),
			Escape:     res.Escape,
			LineEnding: lineEnding(path),
			Header:     &header,
		},
	}
	names := make([]string, len(res.Columns))
	for i, c := range res.Columns {
		names[i] = c.Target
		if res.Header && strings.TrimSpace(c.Name) != "" {
			names[i] = strings.TrimSpace(c.Name)
		}
	}
	names, _ = engine.NormalizeHeader(names)

	for i, c := range res.Columns {
		col := config.TargetColumn{Name: names[i], Type: c.Type}
		switch c.Type {
		case "date", "datetime", "time":
			if len(c.Formats) > 0 {
				col.Format = c.Formats[0]
			}
		case "decimal":
			col.Scale = c.Scale
			if c.DecimalSep != "" && c.DecimalSep != "." {
				col.DecimalSep = config.Scalar(c.DecimalSep)
			}
			col.ThousandsSep = config.Scalar(c.ThousandsSep)
		case "integer":
			col.ThousandsSep = config.Scalar(c.ThousandsSep)
		case "boolean":
			if len(c.TrueValues) > 0 && len(c.FalseValues) > 0 {
				tv, fv := config.Scalar(c.TrueValues[0]), config.Scalar(c.FalseValues[0])
				col.TrueValue, col.FalseValue = &tv, &fv
			}
		}
		t.Columns = append(t.Columns, col)
	}
	applyDefaults(t)
	return &TargetDraft{Target: t, Detection: res}, nil
}

// lineEnding tells how the example ends its lines: "lf" when its first
// line ends with a bare "\n", "crlf" otherwise (the default).
func lineEnding(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return config.LineEndings[0]
	}
	defer f.Close()
	head := make([]byte, 64<<10)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if i := bytes.IndexByte(head, '\n'); i >= 0 && (i == 0 || head[i-1] != '\r') {
		// UTF-16 writes "\r\n" as "\r\x00\n" or "\x00\r\x00\n".
		if i < 2 || head[i-2] != '\r' {
			return "lf"
		}
	}
	return config.LineEndings[0]
}

// LoadTarget reads a target file, with its defaults applied so the editor
// shows every option.
func LoadTarget(path string) (*config.Target, error) {
	t, err := config.LoadTarget(path)
	if err != nil {
		return nil, err
	}
	applyDefaults(t)
	return t, nil
}

// applyDefaults fills a target's unset options, as a mapping does.
func applyDefaults(t *config.Target) {
	(&config.Mapping{Target: config.TargetRef{Inline: t}}).ApplyDefaults()
}

// CheckTarget validates a target on its own. It is compiled within a
// mapping that reads every column from a source column of the same name,
// so that type options are checked too; only the target's issues are kept.
func CheckTarget(t *config.Target) []Issue {
	m := &config.Mapping{
		Version: config.Version,
		Name:    "target",
		Target:  config.TargetRef{Inline: t},
		Columns: map[string]config.Rule{},
	}
	for _, c := range t.Columns {
		if c.Name != "" {
			src := config.Scalar(c.Name)
			m.Columns[c.Name] = config.Rule{Source: &src}
		}
	}
	_, issues := compile(m)
	return slices.DeleteFunc(issues, func(i Issue) bool { return !strings.HasPrefix(i.Path, "target") })
}

// SaveTarget writes a target file. The target must be valid.
func SaveTarget(t *config.Target, path string) ([]Issue, error) {
	issues := CheckTarget(t)
	if slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == "error" }) {
		return issues, errors.New("the target has errors: fix them before saving")
	}
	data, err := config.FormatTarget(t)
	if err != nil {
		return issues, err
	}
	return issues, writeFile(path, data)
}
