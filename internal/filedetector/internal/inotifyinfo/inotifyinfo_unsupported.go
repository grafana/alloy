//go:build !linux

package inotifyinfo

import (
	"log/slog"
)

// Inotify is a Linux-specific mechanism.
func DiagnosticsJson(_ *slog.Logger) string {
	return ""
}
