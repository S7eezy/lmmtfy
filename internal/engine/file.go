package engine

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"lmmtfy/internal/report"
)

// FileJob is one conversion, on files.
type FileJob struct {
	Input   string
	Output  string
	Rejects string // "" disables the reject file
	Log     *report.Logger
}

// ConvertFile converts a file. Output and reject files are written to
// temporary files next to their destination and renamed only on success: a
// failed conversion never leaves a partial or stale file behind.
func (p *Plan) ConvertFile(j FileJob) (*report.Summary, error) {
	in, err := os.Open(j.Input)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	out, err := createTemp(j.Output)
	if err != nil {
		return nil, err
	}
	var rej *os.File
	cleanup := func() {
		os.Remove(out.Name())
		if rej != nil {
			os.Remove(rej.Name())
		}
	}

	job := Job{Input: in, Output: out, Log: j.Log}
	if j.Rejects != "" {
		job.Rejects = func() (io.Writer, error) {
			f, err := createTemp(j.Rejects)
			rej = f
			return f, err
		}
	}

	sum, runErr := p.Run(job)
	closeErr := out.Close()
	if rej != nil {
		closeErr = errors.Join(closeErr, rej.Close())
	}
	if runErr == nil {
		runErr = closeErr
	}
	if runErr != nil {
		cleanup()
		return sum, runErr
	}

	if err := os.Rename(out.Name(), j.Output); err != nil {
		cleanup()
		return sum, err
	}
	switch {
	case rej != nil:
		if err := os.Rename(rej.Name(), j.Rejects); err != nil {
			os.Remove(rej.Name())
			return sum, err
		}
	case j.Rejects != "":
		// No row rejected this time: a reject file from a previous run
		// would be misleading.
		if err := os.Remove(j.Rejects); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return sum, err
		}
	}
	return sum, nil
}

// createTemp creates a hidden temporary file in the directory of path, so
// the final rename stays on the same file system.
func createTemp(path string) (*os.File, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return nil, err
	}
	// CreateTemp uses 0600; output files are meant to be shared.
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
