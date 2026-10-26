package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"lmmtfy/internal/exitcode"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no args", nil, exitcode.Usage, "", "Usage:"},
		{"help", []string{"help"}, exitcode.OK, "Usage:", ""},
		{"version", []string{"version"}, exitcode.OK, "lmmtfy dev", ""},
		{"unknown", []string{"frobnicate"}, exitcode.Usage, "", `unknown command "frobnicate"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestConvertCommand(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.csv")
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		// Flags after the positional argument must work too.
		{"flags after input", []string{"convert", "../../testdata/acme-orders/input.csv", "--mapping", "../../testdata/acme-orders/mapping.yaml", "-o", out}, exitcode.OK},
		{"flags before input", []string{"convert", "--output", out, "--mapping=../../testdata/acme-orders/mapping.yaml", "../../testdata/acme-orders/input.csv"}, exitcode.OK},
		{"strict with handled errors", []string{"convert", "--strict", "../../testdata/skip-row-rejects/input.csv", "--mapping", "../../testdata/skip-row-rejects/mapping.yaml", "-o", out}, exitcode.HandledErrors},
		{"missing mapping flag", []string{"convert", "in.csv", "-o", out}, exitcode.Usage},
		{"unknown flag", []string{"convert", "--nope"}, exitcode.Usage},
		{"input not found", []string{"convert", "nope.csv", "--mapping", "../../testdata/acme-orders/mapping.yaml", "-o", out}, exitcode.IO},
		{"output is input", []string{"convert", "../../testdata/acme-orders/input.csv", "--mapping", "../../testdata/acme-orders/mapping.yaml", "-o", "../../testdata/acme-orders/input.csv"}, exitcode.Usage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\nstderr: %s", code, tt.wantCode, stderr.String())
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{"mapping only", []string{"validate", "--mapping", "../../testdata/acme-orders/mapping.yaml"}, exitcode.OK, "valid, 0 errors, 0 warnings"},
		{"with input", []string{"validate", "--mapping", "../../testdata/acme-orders/mapping.yaml", "--input", "../../testdata/acme-orders/input.csv"}, exitcode.OK, "order_id    ← N° Commande"},
		{"optional missing", []string{"validate", "--mapping", "../../testdata/optional-missing/mapping.yaml", "--input", "../../testdata/optional-missing/input.csv"}, exitcode.OK, "missing, omitted from output"},
		{"required missing", []string{"validate", "--mapping", "../../testdata/missing-required-column/mapping.yaml", "--input", "../../testdata/missing-required-column/input.csv"}, exitcode.FileFailed, "MISSING (required)"},
		{"invalid", []string{"validate", "--mapping", "../../testdata/invalid-config/mapping.yaml"}, exitcode.Usage, "invalid, 6 errors"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\nstderr: %s", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout does not contain %q:\n%s", tt.wantStdout, stdout.String())
			}
		})
	}
}

func TestInspectAndInitCommands(t *testing.T) {
	input := "../../testdata/acme-orders/input.csv"
	var stdout, stderr bytes.Buffer

	if code := run([]string{"inspect", input, "--json"}, &stdout, &stderr); code != exitcode.OK {
		t.Fatalf("inspect --json: exit %d: %s", code, stderr.String())
	}
	var res struct {
		Delimiter string `json:"delimiter"`
		Columns   []any  `json:"columns"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil || res.Delimiter != ";" || len(res.Columns) != 5 {
		t.Errorf("inspect --json: %v, %+v", err, res)
	}

	mapping := filepath.Join(t.TempDir(), "m.yaml")
	if code := run([]string{"init", input, "-o", mapping, "--name", "acme"}, &stdout, &stderr); code != exitcode.OK {
		t.Fatalf("init: exit %d: %s", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"init", input, "-o", mapping}, &stdout, &stderr); code != exitcode.Usage || !strings.Contains(stderr.String(), "--force") {
		t.Errorf("init over existing file: exit %d, stderr %q", code, stderr.String())
	}
	if code := run([]string{"init", input, "-o", mapping, "--force"}, &stdout, &stderr); code != exitcode.OK {
		t.Errorf("init --force: exit %d", code)
	}
	if code := run([]string{"inspect", "nope.csv"}, &stdout, &stderr); code != exitcode.IO {
		t.Errorf("inspect missing file: exit %d", code)
	}
}
