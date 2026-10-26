// Command runner is the template for standalone converters. It is never
// distributed on its own: `lmmtfy build` writes a mapping into its payload slot
// to produce a converter such as acme-converter.exe.
package main

import (
	"fmt"
	"os"

	"lmmtfy/internal/exitcode"
	"lmmtfy/internal/version"
)

func main() {
	// M3: read the mapping from the payload slot and run the engine.
	fmt.Fprintf(os.Stderr, "lmmtfy converter %s: no mapping embedded.\n", version.Version)
	fmt.Fprintln(os.Stderr, "This binary is a template; create a converter with `lmmtfy build`.")
	os.Exit(exitcode.Usage)
}
