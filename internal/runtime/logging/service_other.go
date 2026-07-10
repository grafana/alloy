//go:build !windows

package logging

// isWindowsService always returns false on non-Windows platforms. Stored as
// a var so tests can stub it.
var isWindowsService = func() bool {
	return false
}
