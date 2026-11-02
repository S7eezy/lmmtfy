package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// allowSandbox lets the WebView2 sandbox read a Fixed Version runtime.
// Since runtime 120, its renderer runs in an AppContainer on Windows 10,
// which cannot read a folder unpacked by hand: Microsoft documents these
// two grants (ALL RESTRICTED APPLICATION PACKAGES, ALL APPLICATION
// PACKAGES). The user owns the folder, so no admin rights are needed.
// Best effort, done once: a marker file remembers it.
func allowSandbox(dir string) {
	marker := filepath.Join(dir, ".lmmtfy-sandbox-access")
	if isFile(marker) {
		return
	}
	for _, sid := range []string{"*S-1-15-2-2", "*S-1-15-2-1"} {
		cmd := exec.Command("icacls", dir, "/grant", sid+":(OI)(CI)(RX)")
		// No console window flashing from a GUI program.
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
		if cmd.Run() != nil {
			return
		}
	}
	_ = os.WriteFile(marker, nil, 0o644)
}
