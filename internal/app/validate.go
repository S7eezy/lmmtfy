package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"lmmtfy/internal/config"
	"lmmtfy/internal/engine"
	"lmmtfy/internal/exitcode"
)

// Validate checks a mapping and, when input is not empty, the header of an
// input file against it. It returns the process exit code.
func Validate(name, mappingPath, input string, stdout, stderr io.Writer) int {
	m, err := config.LoadMapping(mappingPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitcode.Usage
	}
	plan, issues := engine.Compile(m)

	errorsCount, warnings := 0, 0
	for _, is := range issues {
		if is.Severity == config.SeverityError {
			errorsCount++
		} else {
			warnings++
		}
	}
	targetName := m.Target.Inline.Name
	if targetName == "" {
		targetName = "inline"
	}
	status := "valid"
	if plan == nil {
		status = "invalid"
	}
	fmt.Fprintf(stdout, "mapping %s (target %s): %s, %s, %s\n", m.Name, targetName, status,
		plural(errorsCount, "error"), plural(warnings, "warning"))
	for _, is := range issues {
		fmt.Fprintf(stdout, "  %s\n", is)
	}
	if plan == nil {
		return exitcode.Usage
	}
	if input == "" {
		return exitcode.OK
	}

	f, err := os.Open(input)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitcode.IO
	}
	defer f.Close()
	check, err := plan.CheckInput(f)
	if err != nil {
		var fe *engine.FileError
		if errors.As(err, &fe) {
			fmt.Fprintf(stderr, "%s: %s: %v\n", name, input, fe)
			return exitcode.FileFailed
		}
		fmt.Fprintf(stderr, "%s: %s: %v\n", name, input, err)
		return exitcode.IO
	}

	fmt.Fprintf(stdout, "\ninput %s:\n", input)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, c := range check.Columns {
		switch c.Status {
		case "found":
			fmt.Fprintf(tw, "  %s\t← %s\t\n", c.Target, c.Source)
		case "constant":
			fmt.Fprintf(tw, "  %s\t(no source: default or empty)\t\n", c.Target)
		case "missing":
			what := map[string]string{
				"empty":   "missing, left empty",
				"default": "missing, default used",
				"omitted": "missing, omitted from output",
				"error":   "MISSING (required)",
			}[c.Action]
			fmt.Fprintf(tw, "  %s\t← %s\t%s\n", c.Target, c.Source, what)
		}
	}
	tw.Flush()
	if len(check.Unused) > 0 {
		fmt.Fprintf(stdout, "  dropped source columns: %s\n", strings.Join(check.Unused, ", "))
	}
	if !check.OK {
		fmt.Fprintf(stdout, "input is not compatible with the mapping: required source columns are missing\n")
		return exitcode.FileFailed
	}
	return exitcode.OK
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
