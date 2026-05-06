//go:build !windows

package logging

// isWindowsService always returns false on non-Windows platforms.
func isWindowsService() bool {
	return false
}
