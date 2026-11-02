package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Windows runs Studio in the Microsoft Edge WebView2 runtime. Air-gapped
// machines may lack it; two ways out, neither needing admin rights:
//
//   - Microsoft's Evergreen Standalone Installer, run without elevation,
//     installs the runtime for the current user.
//   - Microsoft's Fixed Version runtime, unpacked next to the executable,
//     is used without installing anything.

// runtimeExe is the file that marks a WebView2 runtime folder.
const runtimeExe = "msedgewebview2.exe"

// fixedRuntime finds a Fixed Version runtime next to the executable, in
// dir. It accepts the layouts people end up with: a "webview2" folder
// holding the runtime or the folder unpacked from Microsoft's .cab, or that
// unpacked folder (Microsoft.WebView2.FixedVersionRuntime.<version>.<arch>)
// itself. A "webview2" folder without a runtime is still returned, so that
// Studio explains what is wrong with it instead of ignoring it.
func fixedRuntime(dir string) string {
	own := filepath.Join(dir, "webview2")
	unpacked, _ := filepath.Glob(filepath.Join(dir, "Microsoft.WebView2.FixedVersionRuntime.*"))
	nested, _ := filepath.Glob(filepath.Join(own, "*"))
	slices.Sort(unpacked)
	slices.Sort(nested)
	for _, candidate := range slices.Concat([]string{own}, nested, unpacked) {
		if isFile(filepath.Join(candidate, runtimeExe)) {
			return candidate
		}
	}
	if isDir(own) {
		return own
	}
	return ""
}

// webview2Messages are the messages Windows shows when the runtime is
// missing or unusable: Wails' own only offer to download it, which an
// air-gapped machine cannot do.
func webview2Messages(dir string) *windows.Messages {
	offline := `Without internet access: on a connected PC, download the "Evergreen Standalone Installer" (x64) from https://developer.microsoft.com/microsoft-edge/webview2, copy it to this PC and run it. It installs for your account only, without admin rights.`
	m := windows.DefaultMessages()
	m.MissingRequirements = "lmmtfy Studio needs WebView2"
	m.InstallationRequired = "lmmtfy Studio needs the Microsoft Edge WebView2 runtime, which is not installed.\n\n" +
		"With internet access: press OK to download and install it.\n\n" + offline + " Then press Cancel here."
	m.UpdateRequired = "lmmtfy Studio needs a newer Microsoft Edge WebView2 runtime.\n\n" +
		"With internet access: press OK to download and install it.\n\n" + offline + " Then press Cancel here."
	m.FailedToInstall = "The WebView2 runtime could not be installed.\n\n" + offline
	m.ContactAdmin = "lmmtfy Studio needs the Microsoft Edge WebView2 runtime, which is not installed.\n\n" + offline
	m.InvalidFixedWebview2 = fmt.Sprintf("The WebView2 runtime next to lmmtfy Studio cannot be used: %s is missing, the runtime is older than version 94, or it is on a network drive.\n\n"+
		"Unpack Microsoft's Fixed Version runtime with:\n  expand <file>.cab -F:* \"%s\"\n"+
		"or delete the webview2 folder to use an installed runtime.", runtimeExe, dir)
	return m
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
