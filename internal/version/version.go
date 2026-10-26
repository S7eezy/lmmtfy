// Package version holds build information injected at link time.
package version

// Version is the engine version, stamped by the Makefile with
// -ldflags "-X lmmtfy/internal/version.Version=...". Plain `go build` leaves "dev".
var Version = "dev"
