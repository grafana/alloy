//go:build !windows

package logging

import (
	"context"
	"errors"
	"log/slog"
)

var errNotSupported = errors.New("windows event log not supported on this platform")

// newWindowsEventLogHandler creates a new Windows Event Log handler.
// This is a stub implementation for non-Windows platforms.
func newWindowsEventLogHandler(serviceName string, level slog.Leveler, replacer func(groups []string, a slog.Attr) slog.Attr) (*windowsEventLogHandler, error) {
	return nil, errNotSupported
}

// windowsEventLogHandler is a stub for non-Windows platforms.
type windowsEventLogHandler struct{}

var _ slog.Handler = (*windowsEventLogHandler)(nil)

// Enabled is a stub implementation.
func (h *windowsEventLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return false
}

// Handle is a stub implementation.
func (h *windowsEventLogHandler) Handle(ctx context.Context, r slog.Record) error {
	return errNotSupported
}

// WithAttrs is a stub implementation.
func (h *windowsEventLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup is a stub implementation.
func (h *windowsEventLogHandler) WithGroup(name string) slog.Handler {
	return h
}

// Close is a stub implementation.
func (h *windowsEventLogHandler) Close() error {
	return nil
}
