package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixedRuntime(t *testing.T) {
	runtime := func(dir string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, runtimeExe), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const unpacked = "Microsoft.WebView2.FixedVersionRuntime.130.0.2849.80.x64"
	tests := []struct {
		name   string
		layout func(dir string)
		want   string // relative to dir; "" for none
	}{
		{"nothing", func(string) {}, ""},
		{"webview2 folder", func(d string) { runtime(filepath.Join(d, "webview2")) }, "webview2"},
		{"unpacked into webview2", func(d string) { runtime(filepath.Join(d, "webview2", unpacked)) }, filepath.Join("webview2", unpacked)},
		{"unpacked next to the exe", func(d string) { runtime(filepath.Join(d, unpacked)) }, unpacked},
		{"webview2 wins", func(d string) {
			runtime(filepath.Join(d, unpacked))
			runtime(filepath.Join(d, "webview2"))
		}, "webview2"},
		{"any folder name", func(d string) { runtime(filepath.Join(d, "MicrosoftWebView2Runtime")) }, "MicrosoftWebView2Runtime"},
		{"two levels down", func(d string) { runtime(filepath.Join(d, "wv", unpacked)) }, filepath.Join("wv", unpacked)},
		{"too deep", func(d string) { runtime(filepath.Join(d, "a", "b", "c")) }, ""},
		{"empty webview2 is reported", func(d string) { os.Mkdir(filepath.Join(d, "webview2"), 0o755) }, "webview2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.layout(dir)
			want := ""
			if tt.want != "" {
				want = filepath.Join(dir, tt.want)
			}
			if got := fixedRuntime(dir); got != want {
				t.Errorf("fixedRuntime = %q, want %q", got, want)
			}
		})
	}
}
