package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recorder captures Errorf calls instead of failing the real test, so we can
// check that Compare reports mismatches.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func newCase(t *testing.T, files map[string]string) Case {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Case{Name: "case", Dir: dir}
}

func TestCompareMatch(t *testing.T) {
	c := newCase(t, map[string]string{"output.csv": "a,b\r\n1,2\r\n"})
	r := &recorder{TB: t}
	c.Compare(r, "output.csv", []byte("a,b\r\n1,2\r\n"))
	if len(r.errors) != 0 {
		t.Errorf("unexpected errors: %v", r.errors)
	}
}

func TestCompareMismatchReportsLine(t *testing.T) {
	c := newCase(t, map[string]string{"output.csv": "a,b\r\n1,2\r\n"})
	r := &recorder{TB: t}
	c.Compare(r, "output.csv", []byte("a,b\r\n1,2\n")) // LF instead of CRLF on line 2
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], "line 2") {
		t.Errorf("errors = %v, want one mentioning line 2", r.errors)
	}
}

func TestCompareMissingFile(t *testing.T) {
	c := newCase(t, nil)
	r := &recorder{TB: t}
	c.Compare(r, "output.csv", []byte("x"))
	if len(r.errors) != 1 || !strings.Contains(r.errors[0], UpdateEnv) {
		t.Errorf("errors = %v, want one mentioning %s", r.errors, UpdateEnv)
	}
}

func TestCompareUpdateWritesFile(t *testing.T) {
	t.Setenv(UpdateEnv, "1")
	c := newCase(t, map[string]string{"output.csv": "old"})
	c.Compare(t, "output.csv", []byte("new"))
	if got := string(c.Read(t, "output.csv")); got != "new" {
		t.Errorf("file = %q, want %q", got, "new")
	}
}

func TestCasesSkipsUnderscoreAndFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"b-case", "a-case", "_parked"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, c := range Cases(t, root) {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "a-case,b-case" {
		t.Errorf("cases = %s, want a-case,b-case", got)
	}
}

func TestDirFindsRepositoryTestdata(t *testing.T) {
	dir := Dir(t)
	if filepath.Base(dir) != "testdata" {
		t.Errorf("Dir = %s", dir)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "go.mod")); err != nil {
		t.Errorf("no go.mod next to %s", dir)
	}
}
