// Package build creates standalone converters: a runner template for the
// chosen platform with a mapping written into its payload slot.
//
// It is separate from package app because it embeds the runner templates:
// the runner itself uses app, and must not contain other runners.
package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lmmtfy/internal/app"
	"lmmtfy/internal/config"
	"lmmtfy/internal/engine"
	"lmmtfy/internal/exitcode"
	"lmmtfy/internal/payload"
	"lmmtfy/internal/runners"
	"lmmtfy/internal/version"
)

// Request is `lmmtfy build` as asked on the command line.
type Request struct {
	Name    string // program name
	Mapping string // mapping file
	OS      string
	Arch    string // "" = amd64, or arm64 for darwin
	Output  string // ".exe" is added for windows when missing
	Stdout  io.Writer
	Stderr  io.Writer
	Now     func() time.Time
}

// Run builds a converter and returns the process exit code.
func Run(req Request) int {
	fail := func(code int, format string, args ...any) int {
		fmt.Fprintf(req.Stderr, "%s: %s\n", req.Name, fmt.Sprintf(format, args...))
		return code
	}

	target := runners.Target{OS: req.OS, Arch: req.Arch}
	if target.Arch == "" {
		target.Arch = "amd64"
		if target.OS == "darwin" {
			target.Arch = "arm64"
		}
	}
	supported := false
	var names []string
	for _, t := range runners.Supported {
		names = append(names, t.String())
		supported = supported || t == target
	}
	if !supported {
		return fail(exitcode.Usage, "unsupported platform %s (supported: %s)", target, strings.Join(names, ", "))
	}

	plan, _, code := app.LoadPlan(req.Name, req.Mapping, req.Stderr)
	if plan == nil {
		return code
	}
	template, err := runners.Get(target)
	if err != nil {
		return fail(exitcode.IO, "%v", err)
	}
	data, err := Converter(template, plan, target, req.Now())
	if err != nil {
		return fail(exitcode.Usage, "%v", err)
	}

	output := req.Output
	if target.OS == "windows" && !strings.EqualFold(filepath.Ext(output), ".exe") {
		output += ".exe"
	}
	if err := os.WriteFile(output, data, 0o755); err != nil {
		return fail(exitcode.IO, "%v", err)
	}
	fmt.Fprintf(req.Stdout, "built %s for %s: mapping %s (sha256 %s…), %.1f MB\n",
		output, target, plan.Mapping.Name, plan.Hash[:12], float64(len(data))/(1<<20))
	return exitcode.OK
}

// Converter writes plan's mapping into a runner template. macOS converters
// get their ad-hoc signature regenerated.
func Converter(template []byte, plan *engine.Plan, target runners.Target, now time.Time) ([]byte, error) {
	mapping, err := config.Marshal(plan.Mapping)
	if err != nil {
		return nil, err
	}
	meta := payload.Metadata{
		Mapping:       plan.Mapping.Name,
		EngineVersion: version.Version,
		Platform:      target.String(),
		BuiltAt:       now.UTC().Format(time.RFC3339),
		BuiltBy:       "lmmtfy " + version.Version,
	}
	data, err := payload.Patch(template, meta, mapping)
	if err != nil {
		return nil, err
	}
	if err := payload.Resign(data); err != nil {
		return nil, err
	}
	return data, payload.VerifySignature(data)
}
