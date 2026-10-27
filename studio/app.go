package main

import (
	"context"
	"path/filepath"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/S7eezy/lmmtfy/internal/config"
	"github.com/S7eezy/lmmtfy/internal/studio"
	"github.com/S7eezy/lmmtfy/internal/version"
)

// App is bound to the frontend: each exported method can be called from
// JavaScript and returns a promise. Methods only forward to package studio,
// plus native file dialogs.
type App struct {
	ctx   context.Context
	dirty atomic.Bool // the editor has unsaved changes
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// SetDirty is called by the editor whenever its unsaved state changes.
func (a *App) SetDirty(dirty bool) { a.dirty.Store(dirty) }

// beforeClose asks before quitting with unsaved changes. Returning true
// keeps the window open.
func (a *App) beforeClose(ctx context.Context) bool {
	if !a.dirty.Load() {
		return false
	}
	answer, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Unsaved changes",
		Message:       "There are unsaved changes. Quit and discard them?",
		Buttons:       []string{"Discard and quit", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	return err != nil || answer != "Discard and quit" && answer != "Yes"
}

// File dialog filters. Patterns must be plain extensions ("*.yaml"): on
// macOS each one becomes a file type, and an extension with a dot in it
// ("*.target.yaml") has none, which makes the dialog abort the app.
var filters = map[string][]runtime.FileFilter{
	"csv":     {{DisplayName: "CSV files (*.csv, *.txt, *.tsv)", Pattern: "*.csv;*.txt;*.tsv"}, {DisplayName: "All files", Pattern: "*"}},
	"mapping": {{DisplayName: "Mappings (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"}, {DisplayName: "All files", Pattern: "*"}},
	"target":  {{DisplayName: "Targets (*.target.yaml, *.yaml)", Pattern: "*.yaml;*.yml"}, {DisplayName: "All files", Pattern: "*"}},
	"any":     {{DisplayName: "All files", Pattern: "*"}},
}

// ChooseFile asks for a file to open; "" when cancelled. kind is csv,
// mapping, target or any.
func (a *App) ChooseFile(title, kind string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: title, Filters: filters[kind]})
}

// ChooseSaveFile asks where to save a file; "" when cancelled.
func (a *App) ChooseSaveFile(title, defaultPath, kind string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            title,
		DefaultDirectory: filepath.Dir(defaultPath),
		DefaultFilename:  filepath.Base(defaultPath),
		Filters:          filters[kind],
	})
}

// ChooseDirectory asks for a directory; "" when cancelled.
func (a *App) ChooseDirectory(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
}

func (a *App) Open(path string) (*studio.Opened, error) { return studio.Open(path) }

func (a *App) ReadRaw(path string, in config.Input) (*studio.Raw, error) {
	return studio.ReadRaw(path, in)
}

func (a *App) Check(m config.Mapping) []studio.Issue { return studio.Check(&m) }

func (a *App) Preview(path string, m config.Mapping) (*studio.Result, error) {
	return studio.PreviewOutput(path, &m)
}

func (a *App) LoadMapping(path string) (*studio.Loaded, error) { return studio.LoadMapping(path) }

func (a *App) LoadTarget(path string) (*config.Target, error) { return studio.LoadTarget(path) }

func (a *App) DraftTarget(path string) (*studio.TargetDraft, error) { return studio.DraftTarget(path) }

func (a *App) CheckTarget(t config.Target) []studio.Issue { return studio.CheckTarget(&t) }

func (a *App) SaveTarget(t config.Target, path string) ([]studio.Issue, error) {
	return studio.SaveTarget(&t, path)
}

func (a *App) SaveMapping(m config.Mapping, path, targetPath string, writeTarget bool) ([]studio.Issue, error) {
	return studio.SaveMapping(&m, path, targetPath, writeTarget)
}

func (a *App) Convert(m config.Mapping, input, output string) (*studio.Result, error) {
	return studio.Convert(&m, input, output)
}

func (a *App) Platforms() []studio.Platform { return studio.Platforms() }

// Export builds a converter for one platform into dir and returns its path.
func (a *App) Export(m config.Mapping, goos, arch, dir string) (string, error) {
	name := studio.ConverterName(&m, goos, arch)
	return studio.Export(&m, goos, arch, filepath.Join(dir, name))
}

func (a *App) Exists(path string) bool { return studio.Exists(path) }

func (a *App) Version() string { return version.Version }

func (a *App) Recent() []studio.Recent { return studio.RecentFiles() }

func (a *App) Remember(kind, path string) error { return studio.Remember(kind, path) }
