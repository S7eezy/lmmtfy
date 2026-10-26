package app

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"lmmtfy/internal/exitcode"
)

// NewFlagSet returns a flag set printing usage ("Usage: <usage>") and
// errors to stderr.
func NewFlagSet(name, usage string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s\n\nOptions:\n", usage)
		fs.PrintDefaults()
	}
	return fs
}

// ParseArgs parses flags placed anywhere among the positional arguments:
// Go's flag package stops at the first positional one, so `convert in.csv
// -o x` would otherwise ignore -o. It returns the positional arguments, and
// an exit code >= 0 when the command must stop (help shown, bad flag).
func ParseArgs(fs *flag.FlagSet, args []string) ([]string, int) {
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
