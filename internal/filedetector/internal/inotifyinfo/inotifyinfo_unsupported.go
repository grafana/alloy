//go:build !linux

package inotifyinfo

import "github.com/go-kit/log"

// Inotify is a Linux-specific mechanism.
func DiagnosticsJson(logger log.Logger) string {
	return ""
}
