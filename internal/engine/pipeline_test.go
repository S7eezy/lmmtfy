package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/golden"
	"github.com/S7eezy/lmmtfy/internal/report"
)

type runResult struct {
	output, rejects, log []byte
	summary              report.Summary
	err                  string
}

func runCase(t *testing.T, plan *Plan, input []byte, workers int) runResult {
	t.Helper()
	var out, rejects, log bytes.Buffer
	job := Job{
		Input:   bytes.NewReader(input),
		Output:  &out,
		Rejects: func() (io.Writer, error) { return &rejects, nil },
		Log:     report.NewLogger(&log, report.LevelTrace, "text", nil),
		Workers: workers,
	}
	sum, err := plan.Run(job)
	res := runResult{output: out.Bytes(), rejects: rejects.Bytes(), log: log.Bytes(), summary: *sum}
	res.summary.DurationSeconds = 0
	if err != nil {
		res.err = err.Error()
	}
	return res
}

// TestParallelMatchesSequential runs every golden case with tiny batches and
// several worker counts: output, rejects, logs (trace level, so every row),
// summary and error must be identical to a one-worker run. Golden tests
// check that one-worker run against the expected files.
func TestParallelMatchesSequential(t *testing.T) {
	defer func(size int) { batchSize = size }(batchSize)

	golden.Run(t, golden.Dir(t), func(t *testing.T, c golden.Case) {
		m, err := config.LoadMapping(c.Path("mapping.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		plan, _ := Compile(m)
		if plan == nil {
			t.Skip("invalid configuration case")
		}
		input := c.Read(t, "input.csv")

		batchSize = 1024
		want := runCase(t, plan, input, 1)
		for _, size := range []int{1, 2, 3} {
			for _, workers := range []int{1, 2, 3, 8} {
				batchSize = size
				got := runCase(t, plan, input, workers)
				name := fmt.Sprintf("batch=%d workers=%d", size, workers)
				if !bytes.Equal(got.output, want.output) {
					t.Errorf("%s: output differs:\n%s\nwant:\n%s", name, got.output, want.output)
				}
				if !bytes.Equal(got.rejects, want.rejects) {
					t.Errorf("%s: rejects differ:\n%s\nwant:\n%s", name, got.rejects, want.rejects)
				}
				if !bytes.Equal(got.log, want.log) {
					t.Errorf("%s: log differs:\n%s\nwant:\n%s", name, got.log, want.log)
				}
				if fmt.Sprint(got.summary) != fmt.Sprint(want.summary) || got.err != want.err {
					t.Errorf("%s: summary/error differ:\n%+v %q\nwant:\n%+v %q", name, got.summary, got.err, want.summary, want.err)
				}
			}
		}
	})
}

// A failure in the first batch must stop the pipeline without deadlock or
// goroutine leak, however much input remains.
func TestEarlyFailureStopsPipeline(t *testing.T) {
	defer func(size int) { batchSize = size }(batchSize)
	batchSize = 4

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "m.yaml"), []byte(`
version: 1
name: early
target: { columns: [ { name: n, type: integer } ] }
columns: { n: { source: n } }
`), 0o644)
	m, err := config.LoadMapping(filepath.Join(dir, "m.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	plan, issues := Compile(m)
	if plan == nil {
		t.Fatal(issues)
	}

	var input bytes.Buffer
	input.WriteString("n\nbad\n")
	for i := range 100000 {
		fmt.Fprintf(&input, "%d\n", i)
	}
	for _, workers := range []int{1, 2, 16} {
		res := runCase(t, plan, input.Bytes(), workers)
		if res.err != `line 2, column n: "bad" is not an integer` || res.summary.RowsRead != 1 {
			t.Errorf("workers=%d: err = %q, rows read = %d", workers, res.err, res.summary.RowsRead)
		}
	}
}

// A cancelled conversion (Ctrl+C) stops, and leaves no output file.
func TestCancelLeavesNoOutput(t *testing.T) {
	defer func(size int) { batchSize = size }(batchSize)
	batchSize = 8

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "m.yaml"), []byte(`
version: 1
name: cancel
target: { columns: [ { name: n, type: integer } ] }
columns: { n: { source: n } }
`), 0o644)
	m, _ := config.LoadMapping(filepath.Join(dir, "m.yaml"))
	plan, issues := Compile(m)
	if plan == nil {
		t.Fatal(issues)
	}
	var input bytes.Buffer
	input.WriteString("n\n")
	for i := range 200000 {
		fmt.Fprintf(&input, "%d\n", i)
	}
	in := filepath.Join(dir, "in.csv")
	os.WriteFile(in, input.Bytes(), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := filepath.Join(dir, "out.csv")
	_, err := plan.ConvertFile(FileJob{Input: in, Output: out, Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "m.yaml" && e.Name() != "in.csv" {
			t.Errorf("left behind: %s", e.Name())
		}
	}
}
