// Package golden runs file-based test cases stored under the repository's
// testdata/ directory.
//
// Each case is a directory holding inputs (input.csv, mapping.yaml, ...) and
// expected outputs (output.csv, rejects.csv, report.json, ...). A test produces
// actual outputs and checks them with Case.Compare. Directories starting with
// "_" are skipped, to park a case without deleting it.
//
// To rewrite expected files from actual results, run the tests with
// LMMTFY_UPDATE_GOLDEN=1 (or `make golden-update`), then review the diff.
package golden

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// UpdateEnv is the environment variable that switches Compare to rewrite mode.
// An environment variable rather than a -update flag: `go test ./...` rejects
// flags that some of the tested packages do not define.
const UpdateEnv = "LMMTFY_UPDATE_GOLDEN"

// Case is one test case directory.
type Case struct {
	Name string // directory name, used as the subtest name
	Dir  string // absolute path
}

// Path returns the path of a file inside the case directory.
func (c Case) Path(name string) string { return filepath.Join(c.Dir, name) }

// Read returns the contents of a file inside the case directory.
func (c Case) Read(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(c.Path(name))
	if err != nil {
		t.Fatalf("golden case %s: %v", c.Name, err)
	}
	return b
}

// Compare checks got against the expected file name in the case directory.
// Comparison is byte for byte: encodings, BOMs and line endings matter.
func (c Case) Compare(t testing.TB, name string, got []byte) {
	t.Helper()
	path := c.Path(name)

	if os.Getenv(UpdateEnv) != "" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("golden case %s: %v", c.Name, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("golden case %s: expected file %s is missing (run with %s=1 to create it)", c.Name, name, UpdateEnv)
		return
	}
	if err != nil {
		t.Fatalf("golden case %s: %v", c.Name, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("golden case %s: %s differs, %s", c.Name, name, firstDiff(want, got))
	}
}

// CompareFile compares the file at path with the expected file name. When
// path does not exist, the expected file must not exist either: this is how
// a case says "no output" or "no reject file".
func (c Case) CompareFile(t testing.TB, name, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		c.CompareAbsent(t, name)
		return
	}
	if err != nil {
		t.Fatalf("golden case %s: %v", c.Name, err)
	}
	c.Compare(t, name, got)
}

// CompareAbsent checks that the expected file name does not exist.
func (c Case) CompareAbsent(t testing.TB, name string) {
	t.Helper()
	path := c.Path(name)
	if os.Getenv(UpdateEnv) != "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("golden case %s: %v", c.Name, err)
		}
		return
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("golden case %s: %s is expected but was not produced", c.Name, name)
	}
}

// firstDiff describes the first differing line, line endings included.
func firstDiff(want, got []byte) string {
	wantLines := bytes.SplitAfter(want, []byte("\n"))
	gotLines := bytes.SplitAfter(got, []byte("\n"))
	for i := range max(len(wantLines), len(gotLines)) {
		var w, g []byte
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if !bytes.Equal(w, g) {
			return fmt.Sprintf("first difference at line %d:\n  want %q\n  got  %q", i+1, w, g)
		}
	}
	return "contents differ"
}

// Cases lists the case directories under root, sorted by name.
func Cases(t testing.TB, root string) []Case {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	var cases []Case
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), "_") {
			cases = append(cases, Case{Name: e.Name(), Dir: filepath.Join(root, e.Name())})
		}
	}
	return cases
}

// Run calls fn in a subtest for each case under root. It skips when there is none.
func Run(t *testing.T, root string, fn func(t *testing.T, c Case)) {
	t.Helper()
	cases := Cases(t, root)
	if len(cases) == 0 {
		t.Skipf("no golden cases under %s", root)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { fn(t, c) })
	}
}

// Dir returns the repository's testdata directory, found by walking up from
// the working directory (the tested package's directory) to go.mod.
func Dir(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("golden: go.mod not found above working directory")
		}
		dir = parent
	}
}
