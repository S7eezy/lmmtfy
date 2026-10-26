// Command lmmtfy is the command-line tool: convert, validate, inspect, init and build.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"lmmtfy/internal/exitcode"
	"lmmtfy/internal/runners"
	"lmmtfy/internal/version"
)

const usageText = `lmmtfy — Let Me Map That For You

Usage:
  lmmtfy <command> [arguments]

Commands:
  version   print version and embedded converter platforms
  help      show this help
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
