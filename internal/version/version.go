// Package version holds build information injected at link time.
package version

// Version is the engine version, stamped by the Makefile with
// -ldflags "-X github.com/S7eezy/lmmtfy/internal/version.Version=...".
// A plain `go build` leaves "dev".
var Version = "dev"

// Copyright is the author and licence notice, shown by `lmmtfy version`.
// Studio shows the same notice on its home screen.
const Copyright = "Copyright (c) 2026 Valentin Cavarec. MIT License."
