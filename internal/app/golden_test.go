package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"lmmtfy/internal/golden"
	"lmmtfy/internal/report"
)

// TestGolden runs every case under testdata/ through the same code path as
// `lmmtfy convert`, and compares output, reject file, report, log and exit
// code with the expected files. See testdata/README.md.
func TestGolden(t *testing.T) {
	golden.Run(t, golden.Dir(t), func(t *testing.T, c golden.Case) {
		tmp := t.TempDir()
		output := filepath.Join(tmp, "output.csv")
		reportPath := filepath.Join(tmp, "report.json")
		var stderr bytes.Buffer

		plan, _, code := LoadPlan("lmmtfy", c.Path("mapping.yaml"), &stderr)
		if plan != nil {
			code = Convert(ConvertRequest{
				Name:   "lmmtfy",
				Plan:   plan,
				Input:  c.Path("input.csv"),
				Output: output,
				Report: reportPath,
				Stderr: &stderr,
			})
		}

		c.CompareFile(t, "output.csv", output)
		c.CompareFile(t, "rejects.csv", DefaultRejectsPath(output))
		c.Compare(t, "exit_code", fmt.Appendf(nil, "%d\n", code))
		c.Compare(t, "log.txt", normalizeLog(stderr.String(), c.Dir, tmp))

		if data, err := os.ReadFile(reportPath); err == nil {
			c.Compare(t, "report.json", normalizeReport(t, data))
		} else {
			c.CompareAbsent(t, "report.json")
		}

		leftovers, _ := filepath.Glob(filepath.Join(tmp, ".*.tmp"))
		if len(leftovers) > 0 {
			t.Errorf("temporary files left behind: %v", leftovers)
		}
	})
}

var duration = regexp.MustCompile(`\d+\.\d{2}s\n`)

// normalizeLog removes what changes from run to run: paths and durations.
func normalizeLog(log, caseDir, tmp string) []byte {
	log = strings.ReplaceAll(log, caseDir, "<case>")
	log = strings.ReplaceAll(log, tmp, "<tmp>")
	return []byte(duration.ReplaceAllString(log, "<duration>\n"))
}

func normalizeReport(t *testing.T, data []byte) []byte {
	t.Helper()
	var s report.Summary
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.MappingSHA256 == "" {
		t.Error("report has no mapping hash")
	}
	s.MappingSHA256 = "<sha256>"
	s.DurationSeconds = 0
	out, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	return out
}
