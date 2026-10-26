// Package exitcode defines the process exit codes shared by the lmmtfy CLI and
// standalone converters. Production scripts rely on them: never renumber.
package exitcode

const (
	// OK means success. Handled row errors are allowed unless --strict is set.
	OK = 0
	// HandledErrors means success with handled row errors, returned only with --strict.
	HandledErrors = 1
	// Usage means bad command-line usage or an invalid configuration.
	Usage = 2
	// FileFailed means the input was rejected: fail_file, max_error_rate,
	// max_errors, or a missing required column.
	FileFailed = 3
	// IO means a file could not be read or written.
	IO = 4
)
