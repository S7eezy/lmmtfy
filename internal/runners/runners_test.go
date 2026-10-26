package runners

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestFileNameRoundTrip(t *testing.T) {
	for _, target := range Supported {
		name := FileName(target)
		got, ok := parseFileName(name)
		if !ok || got != target {
			t.Errorf("parseFileName(%q) = %v, %v; want %v, true", name, got, ok, target)
		}
	}
}

func TestParseFileNameRejects(t *testing.T) {
	for _, name := range []string{".gitkeep", "runner", "runner-linux", "runner--amd64", "other-linux-amd64"} {
		if got, ok := parseFileName(name); ok {
			t.Errorf("parseFileName(%q) = %v, true; want false", name, got)
		}
	}
}

func TestWindowsRunnerHasExeSuffix(t *testing.T) {
	if got := FileName(Target{"windows", "amd64"}); got != "runner-windows-amd64.exe" {
		t.Errorf("FileName = %q", got)
	}
}

// The Makefile builds the runners, this package lists them: both must agree.
func TestSupportedMatchesMakefile(t *testing.T) {
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}

	var fromMakefile []string
	for line := range strings.Lines(string(makefile)) {
		if value, ok := strings.CutPrefix(line, "RUNNER_TARGETS :="); ok {
			fromMakefile = strings.Fields(value)
		}
	}
	if fromMakefile == nil {
		t.Fatal("RUNNER_TARGETS not found in Makefile")
	}

	var fromGo []string
	for _, target := range Supported {
		fromGo = append(fromGo, target.String())
	}

	if !slices.Equal(fromMakefile, fromGo) {
		t.Errorf("Makefile RUNNER_TARGETS = %v, runners.Supported = %v", fromMakefile, fromGo)
	}
}
