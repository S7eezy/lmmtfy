// Command release packages built binaries into portable archives: unzip and
// run, no installer, no admin rights. `make dist` builds everything first,
// then runs it:
//
//	go run ./tools/release -version v1.0.0 -dist dist
//
// Input (from the build): dist/lmmtfy-<os>-<arch>[.exe] for the CLI, and
// dist/studio/ holding lmmtfy-studio.exe, "lmmtfy Studio.app" and/or a
// Linux lmmtfy-studio. Output: dist/release/*.zip|*.tar.gz and SHA256SUMS.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type file struct {
	name string // path inside the archive
	src  string // file on disk ("" when data is set)
	data []byte
	exec bool
}

func main() {
	version := flag.String("version", "dev", "version in archive names")
	dist := flag.String("dist", "dist", "build output directory")
	flag.Parse()

	out := filepath.Join(*dist, "release")
	if err := os.MkdirAll(out, 0o755); err != nil {
		log.Fatal(err)
	}
	guide := file{name: "USER_GUIDE.md", src: "docs/USER_GUIDE.md"}
	license := file{name: "LICENSE", src: "LICENSE"}
	examples := []file{
		{name: "examples/acme-orders/input.csv", src: "testdata/acme-orders/input.csv"},
		{name: "examples/acme-orders/mapping.yaml", src: "testdata/acme-orders/mapping.yaml"},
		{name: "examples/acme-orders/target.yaml", src: "testdata/acme-orders/target.yaml"},
		{name: "examples/acme-orders/output.csv", src: "testdata/acme-orders/output.csv"},
	}
	var made []string

	// CLI, one archive per platform.
	clis, _ := filepath.Glob(filepath.Join(*dist, "lmmtfy-*-*"))
	for _, bin := range clis {
		base := strings.TrimSuffix(filepath.Base(bin), ".exe")
		platform := strings.TrimPrefix(base, "lmmtfy-") // windows-amd64
		windows := strings.HasPrefix(platform, "windows")
		exe := "lmmtfy"
		if windows {
			exe += ".exe"
		}
		dir := fmt.Sprintf("lmmtfy-%s-%s", *version, platform)
		files := append([]file{
			{name: exe, src: bin, exec: true},
			{name: "README.txt", data: []byte(cliReadme(*version, exe, windows))},
			guide,
			license,
		}, examples...)
		made = append(made, write(out, dir, windows, files))
	}

	// Studio.
	studio := filepath.Join(*dist, "studio")
	if exe := filepath.Join(studio, "lmmtfy-studio.exe"); exists(exe) {
		dir := fmt.Sprintf("lmmtfy-studio-%s-windows-amd64", *version)
		made = append(made, write(out, dir, true, []file{
			{name: "lmmtfy-studio.exe", src: exe, exec: true},
			{name: "README.txt", data: []byte(studioReadme(*version, "windows"))},
			guide,
			license,
		}))
	}
	if app := filepath.Join(studio, "lmmtfy Studio.app"); exists(app) {
		files := []file{{name: "README.txt", data: []byte(studioReadme(*version, "macos"))}, guide, license}
		filepath.WalkDir(app, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(studio, path)
			info, _ := d.Info()
			files = append(files, file{name: filepath.ToSlash(rel), src: path, exec: info.Mode()&0o111 != 0})
			return nil
		})
		dir := fmt.Sprintf("lmmtfy-studio-%s-macos", *version)
		made = append(made, write(out, dir, true, files))
	}
	if bin := filepath.Join(studio, "lmmtfy-studio"); exists(bin) {
		dir := fmt.Sprintf("lmmtfy-studio-%s-linux-amd64", *version)
		made = append(made, write(out, dir, false, []file{
			{name: "lmmtfy-studio", src: bin, exec: true},
			{name: "README.txt", data: []byte(studioReadme(*version, "linux"))},
			guide,
			license,
		}))
	}

	// Checksums, to verify copies made to air-gapped machines.
	slices.Sort(made)
	var sums strings.Builder
	for _, path := range made {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), filepath.Base(path))
		fmt.Printf("  %-58s %6.1f MB\n", filepath.Base(path), float64(len(data))/(1<<20))
	}
	if err := os.WriteFile(filepath.Join(out, "SHA256SUMS"), []byte(sums.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// write creates dir.zip or dir.tar.gz in out, every file under dir/.
func write(out, dir string, asZip bool, files []file) string {
	ext := ".tar.gz"
	if asZip {
		ext = ".zip"
	}
	path := filepath.Join(out, dir+ext)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// One fixed date: archives depend only on their content.
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	read := func(fl file) []byte {
		if fl.data != nil {
			return fl.data
		}
		data, err := os.ReadFile(fl.src)
		if err != nil {
			log.Fatal(err)
		}
		return data
	}
	mode := func(fl file) os.FileMode {
		if fl.exec {
			return 0o755
		}
		return 0o644
	}

	if asZip {
		zw := zip.NewWriter(f)
		for _, fl := range files {
			h := &zip.FileHeader{Name: dir + "/" + fl.name, Method: zip.Deflate, Modified: when}
			h.SetMode(mode(fl))
			w, err := zw.CreateHeader(h)
			if err != nil {
				log.Fatal(err)
			}
			w.Write(read(fl))
		}
		if err := zw.Close(); err != nil {
			log.Fatal(err)
		}
		return path
	}

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, fl := range files {
		data := read(fl)
		h := &tar.Header{Name: dir + "/" + fl.name, Mode: int64(mode(fl)), Size: int64(len(data)), ModTime: when, Format: tar.FormatPAX}
		if err := tw.WriteHeader(h); err != nil {
			log.Fatal(err)
		}
		if _, err := io.Copy(tw, strings.NewReader(string(data))); err != nil {
			log.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	return path
}

func cliReadme(version, exe string, windows bool) string {
	run := "./" + exe
	if windows {
		run = exe
	}
	return fmt.Sprintf(`lmmtfy %s - Let Me Map That For You
=========================================

Portable: nothing to install, no admin rights needed. Unpack anywhere and run.

Try it on the example:

  %[2]s convert examples/acme-orders/input.csv --mapping examples/acme-orders/mapping.yaml -o out.csv

Start a mapping for your own file:

  %[2]s inspect your-file.csv
  %[2]s init your-file.csv -o your.mapping.yaml

Create a standalone converter for a production machine:

  %[2]s build --mapping your.mapping.yaml --os windows -o your-converter

Run "%[2]s help" for all commands. USER_GUIDE.md explains everything.
Check downloads against SHA256SUMS, published next to the archives.
`, version, run)
}

func studioReadme(version, platform string) string {
	notes := map[string]string{
		"windows": `Run lmmtfy-studio.exe. It uses the Microsoft Edge WebView2 runtime, present on
Windows 11 and up-to-date Windows 10. On a machine without it and without
internet access, unpack Microsoft's "Fixed Version" WebView2 runtime into a
folder named "webview2" next to lmmtfy-studio.exe: Studio uses it from there,
without installing anything.

Windows SmartScreen may warn about an unsigned program the first time:
choose "More info", then "Run anyway".`,
		"macos": `Drag "lmmtfy Studio.app" anywhere (Applications or your home folder) and open
it. If macOS says it cannot verify the developer (downloaded from a
browser), right-click the app, choose Open, then Open again. Only needed the
first time.`,
		"linux": `Run ./lmmtfy-studio. It needs WebKitGTK (libwebkit2gtk-4.1), present on
standard GNOME and KDE desktops.`,
	}
	return fmt.Sprintf(`lmmtfy Studio %s - Let Me Map That For You
=================================================

Portable: nothing to install, no admin rights needed.

%s

Studio opens a CSV file, detects how to read it, and lets you map its
columns onto the format your software expects, with a live preview. It saves
mappings, converts files and exports standalone converters for Windows,
Linux and macOS. Settings and recent files are kept in your own user folder.

USER_GUIDE.md explains everything.
`, version, notes[platform])
}
