// Command lmmtfy is the command-line tool: convert, validate, inspect, init and build.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/S7eezy/lmmtfy/internal/app"
	"github.com/S7eezy/lmmtfy/internal/build"
	"github.com/S7eezy/lmmtfy/internal/detect"
	"github.com/S7eezy/lmmtfy/internal/exitcode"
	"github.com/S7eezy/lmmtfy/internal/runners"
	"github.com/S7eezy/lmmtfy/internal/version"
)

const usageText = `lmmtfy — Let Me Map That For You

Usage:
  lmmtfy <command> [arguments]

Commands:
  convert   convert a CSV file with a mapping
  validate  check a mapping, and optionally an input file against it
  inspect   detect the format and column types of a CSV file
  init      write a draft mapping for a CSV file
  build     create a standalone converter from a mapping
  version   print version and embedded converter platforms
  help      show this help

Run "lmmtfy <command> -h" for the arguments of a command.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the os dependencies, so tests can call it directly.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return exitcode.Usage
	}

	switch args[0] {
	case "convert":
		return cmdConvert(args[1:], stderr, time.Now)
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "inspect":
		return cmdInspect(args[1:], stdout, stderr)
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "build":
		return cmdBuild(args[1:], stdout, stderr)
	case "version":
		return cmdVersion(stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return exitcode.OK
	default:
		fmt.Fprintf(stderr, "lmmtfy: unknown command %q\n\n%s", args[0], usageText)
		return exitcode.Usage
	}
}

func cmdConvert(args []string, stderr io.Writer, now func() time.Time) int {
	fs := newFlagSet("convert", "<input.csv> --mapping <mapping.yaml> -o <output.csv> [options]", stderr)
	mapping := fs.String("mapping", "", "mapping file (required)")
	var output string
	fs.StringVar(&output, "o", "", "output file (required)")
	fs.StringVar(&output, "output", "", "same as -o")
	rejects := fs.String("rejects", "", "reject file (default: <output>.rejects.csv when the mapping enables it)")
	reportPath := fs.String("report", "", "write a JSON summary to this file")
	logLevel := fs.String("log-level", "", "override the mapping's log level: none, errors, warnings, info, trace")
	strict := fs.Bool("strict", false, "exit with code 1 when any row had an error, even if handled")
	workers := fs.Int("workers", 0, "rows are converted on this many CPU cores (default: all)")

	pos, code := app.ParseArgs(fs, args)
	if code >= 0 {
		return code
	}
	if len(pos) != 1 || *mapping == "" || output == "" {
		fmt.Fprintln(stderr, "lmmtfy convert: need one input file, --mapping and -o")
		fs.Usage()
		return exitcode.Usage
	}

	plan, _, code := app.LoadPlan("lmmtfy", *mapping, stderr)
	if plan == nil {
		return code
	}
	return app.Convert(app.ConvertRequest{
		Name:     "lmmtfy",
		Plan:     plan,
		Input:    pos[0],
		Output:   output,
		Rejects:  *rejects,
		Report:   *reportPath,
		LogLevel: *logLevel,
		Strict:   *strict,
		Workers:  *workers,
		Stderr:   stderr,
		Now:      now,
	})
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("validate", "--mapping <mapping.yaml> [--input <input.csv>]", stderr)
	mapping := fs.String("mapping", "", "mapping file (required)")
	input := fs.String("input", "", "also check this input file's header against the mapping")

	pos, code := app.ParseArgs(fs, args)
	if code >= 0 {
		return code
	}
	if len(pos) != 0 || *mapping == "" {
		fmt.Fprintln(stderr, "lmmtfy validate: need --mapping")
		fs.Usage()
		return exitcode.Usage
	}
	return app.Validate("lmmtfy", *mapping, *input, stdout, stderr)
}

func cmdInspect(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("inspect", "<input.csv> [--sample N] [--json]", stderr)
	sample := fs.Int("sample", 10000, "number of rows analysed")
	asJSON := fs.Bool("json", false, "print the result as JSON")

	pos, code := app.ParseArgs(fs, args)
	if code >= 0 {
		return code
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "lmmtfy inspect: need one input file")
		fs.Usage()
		return exitcode.Usage
	}
	return app.Inspect("lmmtfy", pos[0], detect.Options{SampleRows: *sample}, *asJSON, stdout, stderr)
}

func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("init", "<input.csv> -o <mapping.yaml> [--name NAME] [--sample N] [--force]", stderr)
	var output string
	fs.StringVar(&output, "o", "", "mapping file to write (required)")
	fs.StringVar(&output, "output", "", "same as -o")
	name := fs.String("name", "", "mapping name (default: from the input file name)")
	sample := fs.Int("sample", 10000, "number of rows analysed")
	force := fs.Bool("force", false, "overwrite the mapping file if it exists")

	pos, code := app.ParseArgs(fs, args)
	if code >= 0 {
		return code
	}
	if len(pos) != 1 || output == "" {
		fmt.Fprintln(stderr, "lmmtfy init: need one input file and -o")
		fs.Usage()
		return exitcode.Usage
	}
	return app.Init(app.InitRequest{
		Name:        "lmmtfy",
		Input:       pos[0],
		Output:      output,
		MappingName: *name,
		Force:       *force,
		Detect:      detect.Options{SampleRows: *sample},
		Stdout:      stdout,
		Stderr:      stderr,
	})
}

func cmdBuild(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("build", "--mapping <mapping.yaml> --os windows|linux|darwin [--arch amd64|arm64] -o <converter>", stderr)
	mapping := fs.String("mapping", "", "mapping file (required)")
	goos := fs.String("os", "", "platform of the converter: windows, linux or darwin (required)")
	arch := fs.String("arch", "", "processor: amd64 or arm64 (default: amd64, arm64 for darwin)")
	var output string
	fs.StringVar(&output, "o", "", "converter file to write (required; .exe is added for windows)")
	fs.StringVar(&output, "output", "", "same as -o")

	pos, code := app.ParseArgs(fs, args)
	if code >= 0 {
		return code
	}
	if len(pos) != 0 || *mapping == "" || *goos == "" || output == "" {
		fmt.Fprintln(stderr, "lmmtfy build: need --mapping, --os and -o")
		fs.Usage()
		return exitcode.Usage
	}
	return build.Run(build.Request{
		Name:    "lmmtfy",
		Mapping: *mapping,
		OS:      *goos,
		Arch:    *arch,
		Output:  output,
		Stdout:  stdout,
		Stderr:  stderr,
		Now:     time.Now,
	})
}

func cmdVersion(stdout io.Writer) int {
	fmt.Fprintf(stdout, "lmmtfy %s\n", version.Version)

	available := runners.Available()
	if len(available) == 0 {
		fmt.Fprintln(stdout, "converters: none embedded (build with `make build`)")
		return exitcode.OK
	}
	names := make([]string, len(available))
	for i, t := range available {
		names[i] = t.String()
	}
	fmt.Fprintf(stdout, "converters: %s\n", strings.Join(names, ", "))
	return exitcode.OK
}

func newFlagSet(name, synopsis string, stderr io.Writer) *flag.FlagSet {
	return app.NewFlagSet(name, "lmmtfy "+name+" "+synopsis, stderr)
}
