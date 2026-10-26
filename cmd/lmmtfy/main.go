// Command lmmtfy is the command-line tool: convert, validate, inspect, init and build.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"lmmtfy/internal/app"
	"lmmtfy/internal/exitcode"
	"lmmtfy/internal/runners"
	"lmmtfy/internal/version"
)

const usageText = `lmmtfy — Let Me Map That For You

Usage:
  lmmtfy <command> [arguments]

Commands:
  convert   convert a CSV file with a mapping
  validate  check a mapping, and optionally an input file against it
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

	pos, code := parse(fs, args)
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
		Stderr:   stderr,
		Now:      now,
	})
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("validate", "--mapping <mapping.yaml> [--input <input.csv>]", stderr)
	mapping := fs.String("mapping", "", "mapping file (required)")
	input := fs.String("input", "", "also check this input file's header against the mapping")

	pos, code := parse(fs, args)
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
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: lmmtfy %s %s\n\nOptions:\n", name, synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parse parses flags placed anywhere among the positional arguments: Go's
// flag package stops at the first positional one, so `convert in.csv -o x`
// would otherwise ignore -o. It returns the positional arguments, and an
// exit code >= 0 when the command must stop.
func parse(fs *flag.FlagSet, args []string) ([]string, int) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, exitcode.OK
			}
			return nil, exitcode.Usage
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, -1
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}
