package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/S7eezy/lmmtfy/internal/app"
	"github.com/S7eezy/lmmtfy/internal/build"
	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/golden"
	"github.com/S7eezy/lmmtfy/internal/payload"
	"github.com/S7eezy/lmmtfy/internal/report"
	"github.com/S7eezy/lmmtfy/internal/runners"
)

var host = runners.Target{OS: runtime.GOOS, Arch: runtime.GOARCH}

// buildTemplate compiles this runner for a platform, as `make runners` does.
func buildTemplate(t *testing.T, target runners.Target) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "runner")
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.OS, "GOARCH="+target.Arch)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", target, err, msg)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeConverter builds a converter for this machine and writes it as
// dir/name.
func writeConverter(t *testing.T, template []byte, mappingPath, dir, name string) string {
	t.Helper()
	var stderr bytes.Buffer
	plan, _, _ := app.LoadPlan("lmmtfy", mappingPath, &stderr)
	if plan == nil {
		t.Fatalf("mapping: %s", stderr.String())
	}
	data, err := build.Converter(template, plan, host, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

var (
	timestamp = regexp.MustCompile(`(?m)^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ `)
	duration  = regexp.MustCompile(`\d+\.\d{2}s\n`)
)

// TestGoldenConverters builds a real converter for every golden case and
// runs it as a separate process: output, reject file, report, log and exit
// code must be exactly those expected from `lmmtfy convert`.
func TestGoldenConverters(t *testing.T) {
	t.Setenv(golden.UpdateEnv, "") // expected files come from the CLI, never from here
	template := buildTemplate(t, host)

	golden.Run(t, golden.Dir(t), func(t *testing.T, c golden.Case) {
		m, err := config.LoadMapping(c.Path("mapping.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		if plan, _, _ := app.LoadPlan("lmmtfy", c.Path("mapping.yaml"), &stderr); plan == nil {
			// No converter can be built from an invalid mapping.
			if want := strings.TrimSpace(string(c.Read(t, "exit_code"))); want != "2" {
				t.Fatalf("mapping invalid but expected exit code is %s", want)
			}
			return
		}

		tmp := t.TempDir()
		// Named lmmtfy so that messages read exactly like the CLI's.
		converter := writeConverter(t, template, c.Path("mapping.yaml"), tmp, "lmmtfy")
		output := filepath.Join(tmp, "output.csv")
		reportPath := filepath.Join(tmp, "report.json")

		cmd := exec.Command(converter, c.Path("input.csv"), output, "--report", reportPath)
		var log bytes.Buffer
		cmd.Stderr = &log
		code := exitCode(cmd.Run())

		c.Compare(t, "exit_code", []byte(strconv.Itoa(code)+"\n"))
		c.CompareFile(t, "output.csv", output)
		c.CompareFile(t, "rejects.csv", app.DefaultRejectsPath(output))

		text := strings.ReplaceAll(log.String(), c.Dir, "<case>")
		text = strings.ReplaceAll(text, tmp, "<tmp>")
		text = timestamp.ReplaceAllString(text, "")
		c.Compare(t, "log.txt", []byte(duration.ReplaceAllString(text, "<duration>\n")))

		data, err := os.ReadFile(reportPath)
		if err != nil {
			c.CompareAbsent(t, "report.json")
			return
		}
		var s report.Summary
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatal(err)
		}
		if want, _ := config.Hash(m); s.MappingSHA256 != want {
			t.Errorf("report mapping hash %s, want %s (the CLI's)", s.MappingSHA256, want)
		}
		s.MappingSHA256, s.DurationSeconds = "<sha256>", 0
		normalized, _ := s.JSON()
		c.Compare(t, "report.json", normalized)
	})
}

func TestConverterCommands(t *testing.T) {
	template := buildTemplate(t, host)
	dir := t.TempDir()
	mappingPath := "../../testdata/acme-orders/mapping.yaml"
	converter := writeConverter(t, template, mappingPath, dir, "acme-converter")

	run := func(path string, args ...string) (string, string, int) {
		cmd := exec.Command(path, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		code := exitCode(cmd.Run())
		return stdout.String(), stderr.String(), code
	}

	out, _, code := run(converter, "--info")
	if code != 0 || !strings.Contains(out, "mapping         acme-orders") || !strings.Contains(out, "built           2026-01-01T00:00:00Z") {
		t.Errorf("--info: exit %d\n%s", code, out)
	}

	out, _, code = run(converter, "--print-config")
	m, _ := config.LoadMapping(mappingPath)
	want, _ := config.Marshal(m)
	if code != 0 || out != string(want) {
		t.Errorf("--print-config: exit %d, output differs from the resolved mapping", code)
	}

	if _, stderr, code := run(converter); code != 2 || !strings.Contains(stderr, "need an input file and an output file") {
		t.Errorf("no arguments: exit %d, %q", code, stderr)
	}

	// The bare template refuses to run.
	bare := filepath.Join(dir, "template")
	os.WriteFile(bare, template, 0o755)
	if _, stderr, code := run(bare, "in.csv", "out.csv"); code != 2 || !strings.Contains(stderr, "no mapping embedded") {
		t.Errorf("template: exit %d, %q", code, stderr)
	}

	// A damaged converter says so instead of converting with garbage.
	data, _ := os.ReadFile(converter)
	at := bytes.Index(data, payload.Marker())
	data[at+200] ^= 0xFF // inside the compressed mapping
	payload.Resign(data)
	damaged := filepath.Join(dir, "damaged")
	os.WriteFile(damaged, data, 0o755)
	if _, stderr, code := run(damaged, "--info"); code != 2 || !strings.Contains(stderr, "damaged") {
		t.Errorf("damaged: exit %d, %q", code, stderr)
	}
}

// Converters for every platform: the slot is found and filled, and macOS
// signatures are valid again after patching.
func TestCrossPlatformConverters(t *testing.T) {
	var stderr bytes.Buffer
	plan, _, _ := app.LoadPlan("lmmtfy", "../../testdata/acme-orders/mapping.yaml", &stderr)
	if plan == nil {
		t.Fatal(stderr.String())
	}
	for _, target := range runners.Supported {
		t.Run(target.String(), func(t *testing.T) {
			template := buildTemplate(t, target)
			data, err := build.Converter(template, plan, target, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			at := bytes.Index(data, payload.Marker())
			meta, _, err := payload.Decode(data[at : at+payload.SlotSize])
			if err != nil || meta.Platform != target.String() || meta.MappingSHA256 != plan.Hash {
				t.Errorf("decode: %v, %+v", err, meta)
			}
			if err := payload.VerifySignature(data); err != nil {
				t.Errorf("signature: %v", err)
			}
			if target.OS == "darwin" && target.Arch == "arm64" {
				// Patching without re-signing must break the signature,
				// otherwise the check above proves nothing.
				raw, _ := payload.Patch(template, payload.Metadata{}, []byte("x"))
				if payload.VerifySignature(raw) == nil {
					t.Error("unsigned patch passed verification")
				}
			}
			if target == host && runtime.GOOS == "darwin" {
				path := filepath.Join(t.TempDir(), "converter")
				os.WriteFile(path, data, 0o755)
				if msg, err := exec.Command("codesign", "--verify", "--strict", path).CombinedOutput(); err != nil {
					t.Errorf("codesign --verify: %v\n%s", err, msg)
				}
			}
		})
	}
}
