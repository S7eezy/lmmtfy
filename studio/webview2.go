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
// dir: the folder holding msedgewebview2.exe, at most two levels down, so
// that any way of unpacking Microsoft's .cab works (into dir, into a
// "webview2" folder, whatever the folder names). A "webview2" folder is
// looked at first. A "webview2" folder without a runtime is still
// returned, so that Studio explains what is wrong with it instead of
// ignoring it.
func fixedRuntime(dir string) string {
	own := filepath.Join(dir, "webview2")
	for _, pattern := range []string{
		filepath.Join(own, runtimeExe),
		filepath.Join(own, "*", runtimeExe),
		filepath.Join(dir, "*", runtimeExe),
		filepath.Join(dir, "*", "*", runtimeExe),
	} {
		found, _ := filepath.Glob(pattern)
		slices.Sort(found)
		for _, exe := range found {
			if isFile(exe) {
				return filepath.Dir(exe)
			}
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
func webview2Messages(dir, runtime string) *windows.Messages {
	offline := `Without internet access: on a connected PC, download the "Evergreen Standalone Installer" (x64) from https://developer.microsoft.com/microsoft-edge/webview2, copy it to this PC and run it. It installs for your account only, without admin rights.`
	m := windows.DefaultMessages()
	m.MissingRequirements = "lmmtfy Studio needs WebView2"
	m.InstallationRequired = "lmmtfy Studio needs the Microsoft Edge WebView2 runtime, which is not installed.\n\n" +
		"With internet access: press OK to download and install it.\n\n" + offline + " Then press Cancel here."
	m.UpdateRequired = "lmmtfy Studio needs a newer Microsoft Edge WebView2 runtime.\n\n" +
		"With internet access: press OK to download and install it.\n\n" + offline + " Then press Cancel here."
	m.FailedToInstall = "The WebView2 runtime could not be installed.\n\n" + offline
	m.ContactAdmin = "lmmtfy Studio needs the Microsoft Edge WebView2 runtime, which is not installed.\n\n" + offline
	m.InvalidFixedWebview2 = fmt.Sprintf("The WebView2 runtime in %s cannot be used: %s is missing or does not start, the runtime is older than version 94, or it is on a network drive.\n\n"+
		"Unpack Microsoft's Fixed Version runtime (x64) with:\n  expand <file>.cab -F:* \"%s\"\n"+
		"or delete that folder to use an installed runtime.", runtime, runtimeExe, dir)
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
