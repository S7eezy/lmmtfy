// Command lmmtfy-studio is the desktop editor: open a CSV file, set how to
// read it, map its columns onto a target, save the mapping, convert files
// and export standalone converters.
//
// This is a separate Go module so that the rest of lmmtfy builds and tests
// without a web view or cgo. All logic lives in lmmtfy/internal/studio;
// this package only connects it to the window.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := &App{}
	err := wails.Run(&options.App{
		Title:     "lmmtfy Studio",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: a.startup,
		Bind:      []any{a},
	})
	if err != nil {
		log.Fatal(err)
	}
}
