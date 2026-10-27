// Command runner is the template for standalone converters. It is never
// distributed on its own: `lmmtfy build` writes a mapping into its payload
// slot to produce a converter such as acme-converter.exe, which converts
// files with that mapping and nothing else.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/S7eezy/lmmtfy/internal/app"
	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/engine"
	"github.com/S7eezy/lmmtfy/internal/exitcode"
	"github.com/S7eezy/lmmtfy/internal/payload"
	"github.com/S7eezy/lmmtfy/internal/payload/slot"
	"github.com/S7eezy/lmmtfy/internal/version"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr io.Writer) int {
	prog := strings.TrimSuffix(filepath.Base(argv[0]), ".exe")
	meta, mapping, err := slot.Read()
	switch {
	case errors.Is(err, payload.ErrEmpty):
		fmt.Fprintf(stderr, "lmmtfy converter %s: no mapping embedded.\n", version.Version)
		fmt.Fprintln(stderr, "This binary is a template; create a converter with `lmmtfy build`.")
		return exitcode.Usage
	case err != nil:
		fmt.Fprintf(stderr, "%s: the embedded mapping is damaged (%v): rebuild this converter\n", prog, err)
		return exitcode.Usage
	}

	fs := app.NewFlagSet(prog, prog+" <input.csv> <output.csv> [options]", stderr)
	rejects := fs.String("rejects", "", "reject file (default: <output>.rejects.csv when the mapping enables it)")
	reportPath := fs.String("report", "", "write a JSON summary to this file")
	logLevel := fs.String("log-level", "", "override the mapping's log level: none, errors, warnings, info, trace")
	strict := fs.Bool("strict", false, "exit with code 1 when any row had an error, even if handled")
	workers := fs.Int("workers", 0, "rows are converted on this many CPU cores (default: all)")
	info := fs.Bool("info", false, "print the embedded mapping's name, checksum and build information")
	printConfig := fs.Bool("print-config", false, "print the embedded mapping")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s <input.csv> <output.csv> [options]\n\nConverts CSV files with the embedded mapping %q.\n\nOptions:\n", prog, meta.Mapping)
		fs.PrintDefaults()
	}

	pos, code := app.ParseArgs(fs, argv[1:])
	if code >= 0 {
		return code
	}
	switch {
	case *info:
		fmt.Fprintf(stdout, "%s\n  mapping         %s\n  mapping sha256  %s\n  engine          %s\n  platform        %s\n  built           %s by %s\n",
			prog, meta.Mapping, meta.MappingSHA256, meta.EngineVersion, meta.Platform, meta.BuiltAt, meta.BuiltBy)
		return exitcode.OK
	case *printConfig:
		stdout.Write(mapping)
		return exitcode.OK
	case len(pos) != 2:
		fmt.Fprintf(stderr, "%s: need an input file and an output file\n", prog)
		fs.Usage()
		return exitcode.Usage
	}

	m, err := config.ParseMapping(mapping)
	if err != nil {
		fmt.Fprintf(stderr, "%s: embedded mapping: %v\n", prog, err)
		return exitcode.Usage
	}
	m.ApplyDefaults()
	plan, issues := engine.Compile(m)
	if plan == nil {
		fmt.Fprintf(stderr, "%s: embedded mapping: %v\n", prog, issues.Err())
		return exitcode.Usage
	}
	return app.Convert(app.ConvertRequest{
		Name:     prog,
		Plan:     plan,
		Input:    pos[0],
		Output:   pos[1],
		Rejects:  *rejects,
		Report:   *reportPath,
		LogLevel: *logLevel,
		Strict:   *strict,
		Workers:  *workers,
		Stderr:   stderr,
		Now:      time.Now,
	})
}
