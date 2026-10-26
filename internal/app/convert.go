// Package app holds the commands shared by the lmmtfy CLI and standalone
// converters: same behaviour, same messages, same exit codes.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lmmtfy/internal/config"
	"lmmtfy/internal/engine"
	"lmmtfy/internal/exitcode"
	"lmmtfy/internal/report"
)

// ConvertRequest is one conversion as asked on the command line.
type ConvertRequest struct {
	Name     string // program name, prefixes error messages
	Plan     *engine.Plan
	Input    string
	Output   string
	Rejects  string // "" = default path when reject_file is enabled
	Report   string // "" = no JSON report
	LogLevel string // "" = mapping's logging.level
	Strict   bool
	Workers  int // 0 = one per CPU
	Stderr   io.Writer
	Now      func() time.Time // log timestamps; nil = none
}

// Convert runs a conversion and returns the process exit code.
func Convert(req ConvertRequest) int {
	m := req.Plan.Mapping
	fail := func(code int, format string, args ...any) int {
		fmt.Fprintf(req.Stderr, "%s: %s\n", req.Name, fmt.Sprintf(format, args...))
		return code
	}

	levelName := m.Logging.Level
	if req.LogLevel != "" {
		levelName = req.LogLevel
	}
	level, err := report.ParseLevel(levelName)
	if err != nil {
		return fail(exitcode.Usage, "%v", err)
	}

	logOut := req.Stderr
	if m.Logging.File != "" {
		f, err := os.OpenFile(m.Logging.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fail(exitcode.IO, "cannot open log file: %v", err)
		}
		defer f.Close()
		logOut = f
	}
	log := report.NewLogger(logOut, level, m.Logging.Format, req.Now)

	if same, err := sameFile(req.Input, req.Output); err == nil && same {
		return fail(exitcode.Usage, "output %s is the input file", req.Output)
	}

	rejects := req.Rejects
	if rejects == "" && *m.OnError.RejectFile {
		rejects = DefaultRejectsPath(req.Output)
	}

	// Ctrl+C or a service stop cancels the conversion: temporary files are
	// removed and no partial output is left.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info(report.Event{Message: fmt.Sprintf("converting %s with mapping %s", req.Input, m.Name)})
	sum, err := req.Plan.ConvertFile(engine.FileJob{Input: req.Input, Output: req.Output, Rejects: rejects, Log: log, Workers: req.Workers, Context: ctx})

	if sum != nil {
		if *m.Logging.Summary {
			log.Print(sum.Text())
		}
		if req.Report != "" {
			if werr := sum.WriteJSON(req.Report); werr != nil {
				fail(exitcode.IO, "cannot write report: %v", werr)
				if err == nil {
					return exitcode.IO
				}
			}
		}
	}

	var fe *engine.FileError
	switch {
	case errors.Is(err, context.Canceled):
		return fail(exitcode.Interrupted, "interrupted: no output written")
	case errors.As(err, &fe):
		return fail(exitcode.FileFailed, "conversion failed: %v", fe)
	case err != nil:
		return fail(exitcode.IO, "%v", err)
	case req.Strict && sum.RowsWithErrors > 0:
		return fail(exitcode.HandledErrors, "%d rows had errors (--strict)", sum.RowsWithErrors)
	}
	return exitcode.OK
}

// DefaultRejectsPath is out.rejects.csv for out.csv.
func DefaultRejectsPath(output string) string {
	ext := filepath.Ext(output)
	return strings.TrimSuffix(output, ext) + ".rejects" + ext
}

func sameFile(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}

// LoadPlan loads and compiles a mapping file. On failure it prints the
// problems and returns a non-zero exit code.
func LoadPlan(name, path string, stderr io.Writer) (*engine.Plan, config.Issues, int) {
	m, err := config.LoadMapping(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return nil, nil, exitcode.Usage
	}
	plan, issues := engine.Compile(m)
	if plan == nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", name, path, issues.Err())
		return nil, issues, exitcode.Usage
	}
	return plan, issues, exitcode.OK
}
