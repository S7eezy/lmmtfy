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
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := &App{}
	win := &windows.Options{}
	// Air-gapped Windows machines may lack the WebView2 runtime: a Fixed
	// Version runtime next to the executable is used without installation,
	// and the messages say how to get one (see webview2.go).
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		path := fixedRuntime(dir)
		win.Messages = webview2Messages(dir, path)
		if path != "" {
			win.WebviewBrowserPath = path
			allowSandbox(path)
		}
	}
	err := wails.Run(&options.App{
		Title:     "lmmtfy Studio",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     a.startup,
		OnBeforeClose: a.beforeClose,
		Bind:          []any{a},
		DragAndDrop:   &options.DragAndDrop{EnableFileDrop: true},
		Windows:       win,
		Menu:          appMenu(),
		// Without macOS options, Wails disables the zoom button, and with it
		// full screen: the window must say it can zoom.
		Mac: &mac.Options{DisableZoom: false},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// appMenu is the macOS menu bar. Without an Edit menu, macOS text fields
// lose their shortcuts (cut, copy, paste, select all, undo), and without
// the application menu ⌘Q does nothing. The page still sees each shortcut
// first: Studio's own undo answers ⌘Z outside text fields. Windows and
// Linux need none: their shortcuts work without a menu bar.
func appMenu() *menu.Menu {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu(), menu.WindowMenu())
}
