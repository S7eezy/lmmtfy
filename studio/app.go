package main

import (
	"context"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"lmmtfy/internal/config"
	"lmmtfy/internal/studio"
	"lmmtfy/internal/version"
)

// App is bound to the frontend: each exported method can be called from
// JavaScript and returns a promise. Methods only forward to package studio,
// plus native file dialogs.
type App struct {
	ctx context.Context
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

var filters = map[string][]runtime.FileFilter{
	"csv":     {{DisplayName: "CSV files (*.csv, *.txt, *.tsv)", Pattern: "*.csv;*.txt;*.tsv"}, {DisplayName: "All files", Pattern: "*"}},
	"mapping": {{DisplayName: "Mappings (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"}, {DisplayName: "All files", Pattern: "*"}},
	"any":     {{DisplayName: "All files", Pattern: "*"}},
}

// ChooseFile asks for a file to open; "" when cancelled. kind is csv,
// mapping or any.
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

func (a *App) SaveMapping(m config.Mapping, path, targetPath string) ([]studio.Issue, error) {
	return studio.SaveMapping(&m, path, targetPath)
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

func (a *App) Version() string { return version.Version }
