package app

import (
	"fmt"
	"io"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/engine"
	"github.com/S7eezy/lmmtfy/internal/exitcode"
)

// Test replays the saved tests of a mapping. It returns the process exit
// code: OK when every test passes, FileFailed when one fails.
func Test(name, mappingPath string, stdout, stderr io.Writer) int {
	m, err := config.LoadMapping(mappingPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitcode.Usage
	}
	plan, issues := engine.Compile(m)
	if plan == nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, issues.Err())
		return exitcode.Usage
	}
	if len(m.Tests) == 0 {
		fmt.Fprintf(stdout, "mapping %s has no tests\n", m.Name)
		return exitcode.OK
	}

	failed := 0
	for _, r := range plan.RunTests() {
		if r.Passed {
			fmt.Fprintf(stdout, "ok    %s\n", r.Name)
			continue
		}
		failed++
		fmt.Fprintf(stdout, "FAIL  %s\n", r.Name)
		for _, p := range r.Problems {
			fmt.Fprintf(stdout, "      %s\n", p)
		}
	}
	fmt.Fprintf(stdout, "%d passed, %d failed\n", len(m.Tests)-failed, failed)
	if failed > 0 {
		return exitcode.FileFailed
	}
	return exitcode.OK
}
