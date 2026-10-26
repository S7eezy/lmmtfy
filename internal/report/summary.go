package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Summary describes one conversion run. It is printed at the end of the run
// and written as JSON with --report.
type Summary struct {
	Mapping       string `json:"mapping"`
	MappingSHA256 string `json:"mapping_sha256"`
	EngineVersion string `json:"engine_version"`

	Status        string `json:"status"` // "ok" or "failed"
	FailureReason string `json:"failure_reason,omitempty"`

	RowsRead       int64 `json:"rows_read"`
	RowsWritten    int64 `json:"rows_written"`
	RowsRejected   int64 `json:"rows_rejected"`
	RowsWithErrors int64 `json:"rows_with_errors"`

	// Columns lists target columns with at least one error, default or warning.
	Columns         []ColumnStats   `json:"columns,omitempty"`
	MissingOptional []MissingColumn `json:"missing_optional_columns,omitempty"`

	DurationSeconds float64 `json:"duration_seconds"`
}

// ColumnStats counts events for one target column.
type ColumnStats struct {
	Name      string `json:"name"`
	Errors    int64  `json:"errors"`
	Defaulted int64  `json:"defaulted"` // errors replaced by the default (on_error: default)
	Warnings  int64  `json:"warnings"`
}

// MissingColumn is an optional source column absent from the input.
type MissingColumn struct {
	Source string `json:"source"` // header name, or "#N" for a position
	Target string `json:"target"`
	Action string `json:"action"` // "empty", "default" or "omitted"
}

// Text renders the summary for humans.
func (s *Summary) Text() string {
	var b strings.Builder
	if s.Status == "failed" {
		// Rows "written" so far went to a temporary file that was deleted.
		fmt.Fprintf(&b, "summary: failed after %s, no output written, %.2fs\n", plural(s.RowsRead, "row"), s.DurationSeconds)
		fmt.Fprintf(&b, "  reason: %s\n", s.FailureReason)
	} else {
		fmt.Fprintf(&b, "summary: ok, %s read, %d written, %d rejected, %d with errors, %.2fs\n",
			plural(s.RowsRead, "row"), s.RowsWritten, s.RowsRejected, s.RowsWithErrors, s.DurationSeconds)
	}
	for _, c := range s.Columns {
		var parts []string
		if c.Errors > 0 {
			parts = append(parts, plural(c.Errors, "error"))
		}
		if c.Defaulted > 0 {
			parts = append(parts, fmt.Sprintf("%d defaulted", c.Defaulted))
		}
		if c.Warnings > 0 {
			parts = append(parts, plural(c.Warnings, "warning"))
		}
		fmt.Fprintf(&b, "  %s: %s\n", c.Name, strings.Join(parts, ", "))
	}
	return b.String()
}

// JSON renders the summary as indented JSON.
func (s *Summary) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep < > & readable: this is not HTML
	enc.SetIndent("", "  ")
	err := enc.Encode(s)
	return buf.Bytes(), err
}

// WriteJSON writes the summary as indented JSON to path.
func (s *Summary) WriteJSON(path string) error {
	data, err := s.JSON()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func plural(n int64, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
