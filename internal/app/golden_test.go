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

	"github.com/S7eezy/lmmtfy/internal/detect"
	"github.com/S7eezy/lmmtfy/internal/golden"
	"github.com/S7eezy/lmmtfy/internal/report"
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

// TestGoldenDetect runs `inspect` and `init` on every case input and
// compares their output with inspect.txt and init.yaml. The generated
// mapping must then convert its own input without error.
func TestGoldenDetect(t *testing.T) {
	golden.Run(t, golden.Dir(t), func(t *testing.T, c golden.Case) {
		var stdout, stderr bytes.Buffer
		if code := Inspect("lmmtfy", c.Path("input.csv"), detect.Options{}, false, &stdout, &stderr); code != 0 {
			t.Fatalf("inspect exit code %d: %s", code, stderr.String())
		}
		c.Compare(t, "inspect.txt", normalizeLog(stdout.String(), c.Dir, ""))

		tmp := t.TempDir()
		mapping := filepath.Join(tmp, "init.yaml")
		stdout.Reset()
		if code := Init(InitRequest{Name: "lmmtfy", Input: c.Path("input.csv"), Output: mapping, Stdout: &stdout, Stderr: &stderr}); code != 0 {
			t.Fatalf("init exit code %d: %s", code, stderr.String())
		}
		c.CompareFile(t, "init.yaml", mapping)

		plan, _, code := LoadPlan("lmmtfy", mapping, &stderr)
		if plan == nil {
			t.Fatalf("generated mapping: exit code %d: %s", code, stderr.String())
		}
		code = Convert(ConvertRequest{Name: "lmmtfy", Plan: plan, Input: c.Path("input.csv"), Output: filepath.Join(tmp, "out.csv"), Stderr: &stderr})
		if code != 0 {
			t.Errorf("converting with the generated mapping: exit code %d\n%s", code, stderr.String())
		}
	})
}

var duration = regexp.MustCompile(`\d+\.\d{2}s\n`)

// normalizeLog removes what changes from run to run: paths and durations.
func normalizeLog(log, caseDir, tmp string) []byte {
	log = strings.ReplaceAll(log, caseDir, "<case>")
	if tmp != "" {
		log = strings.ReplaceAll(log, tmp, "<tmp>")
	}
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
