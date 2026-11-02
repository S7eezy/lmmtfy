//go:build !windows

package main

// allowSandbox only matters on Windows.
func allowSandbox(string) {}
