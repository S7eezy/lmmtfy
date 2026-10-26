package studio

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Recent is a file opened recently in Studio.
type Recent struct {
	Kind   string    `json:"kind"` // "csv" or "mapping"
	Path   string    `json:"path"`
	Opened time.Time `json:"opened"`
}

const maxRecent = 10

// configDir is where Studio keeps its settings: the user's own config
// directory, never next to the executable or in system paths, so Studio
// needs no admin rights. A variable so tests can redirect it.
var configDir = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lmmtfy"), nil
}

type settings struct {
	Recent []Recent `json:"recent"`
}

func loadSettings() (*settings, string, error) {
	dir, err := configDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "studio.json")
	s := &settings{}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, path, err
	}
	if err == nil {
		// A damaged settings file is not worth an error: start afresh.
		json.Unmarshal(data, s)
	}
	return s, path, nil
}

// RecentFiles lists recently opened files that still exist, newest first.
func RecentFiles() []Recent {
	s, _, err := loadSettings()
	if err != nil {
		return nil
	}
	var out []Recent
	for _, r := range s.Recent {
		if _, err := os.Stat(r.Path); err == nil {
			out = append(out, r)
		}
	}
	return out
}

// Remember records that a file was opened.
func Remember(kind, path string) error {
	s, file, err := loadSettings()
	if err != nil {
		return err
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	s.Recent = slices.DeleteFunc(s.Recent, func(r Recent) bool { return r.Path == path })
	s.Recent = slices.Insert(s.Recent, 0, Recent{Kind: kind, Path: path, Opened: time.Now().UTC()})
	if len(s.Recent) > maxRecent {
		s.Recent = s.Recent[:maxRecent]
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return writeFile(file, data)
}
