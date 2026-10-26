// Package runners gives access to the prebuilt converter templates (cmd/runner)
// compiled into lmmtfy binaries.
//
// `make runners` cross-compiles cmd/runner for every supported platform into the
// bin/ directory next to this file; go:embed then bakes those files into
// whichever binary imports this package. A plain `go build` without
// `make runners` still compiles (bin/ holds a placeholder), but no runner is
// available at run time.
package runners

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

// The "all:" prefix makes go:embed include bin/.gitkeep, so the pattern always
// matches something and the package compiles before any runner is built.
//
//go:embed all:bin
var files embed.FS

// Target is a platform a converter can be built for, in Go's GOOS/GOARCH terms.
type Target struct {
	OS   string
	Arch string
}

func (t Target) String() string { return t.OS + "/" + t.Arch }

// Supported lists the platforms converters can be built for.
// Keep in sync with RUNNER_TARGETS in the Makefile (checked by tests).
var Supported = []Target{
	{"windows", "amd64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "arm64"},
	{"darwin", "amd64"},
}

// FileName is the name of the runner binary for t inside bin/.
func FileName(t Target) string {
	name := "runner-" + t.OS + "-" + t.Arch
	if t.OS == "windows" {
		name += ".exe"
	}
	return name
}

// parseFileName is the inverse of FileName.
func parseFileName(name string) (Target, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSuffix(name, ".exe"), "runner-")
	if !ok {
		return Target{}, false
	}
	goos, goarch, ok := strings.Cut(rest, "-")
	if !ok || goos == "" || goarch == "" {
		return Target{}, false
	}
	return Target{goos, goarch}, true
}

// Get returns the runner binary for t.
func Get(t Target) ([]byte, error) {
	b, err := files.ReadFile("bin/" + FileName(t))
	if err != nil {
		return nil, fmt.Errorf("no converter template embedded for %s (build with `make build`)", t)
	}
	return b, nil
}

// Available lists the targets whose runner is embedded, sorted by file name.
func Available() []Target {
	entries, err := fs.ReadDir(files, "bin")
	if err != nil {
		return nil
	}
	var out []Target
	for _, e := range entries {
		if t, ok := parseFileName(e.Name()); ok {
			out = append(out, t)
		}
	}
	return out
}
