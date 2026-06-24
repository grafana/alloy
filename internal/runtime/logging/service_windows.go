//go:build windows

package logging

import (
	"golang.org/x/sys/windows/svc"
)

// isWindowsService reports whether the current process is running as a Windows
// service.  Stored as a var so tests can stub it.
var isWindowsService = func() bool {
	if ok, err := svc.IsWindowsService(); err == nil && ok {
		return true
	}
	return false
}
