package logging

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/grafana/alloy/internal/runtime/logging/eventlog"
)

// windowsEventLogHandler is a slog.Handler that writes logs to the Windows Event Log.
type windowsEventLogHandler struct {
	el       eventlog.EventLog
	level    slog.Leveler
	attrs    []slog.Attr
	groups   []string
	replacer func(groups []string, a slog.Attr) slog.Attr

	mu sync.Mutex
}

var _ slog.Handler = (*windowsEventLogHandler)(nil)

// newWindowsEventLogHandler creates a new Windows Event Log handler using the given EventLog.
func newWindowsEventLogHandler(el eventlog.EventLog, level slog.Leveler, replacer func(groups []string, a slog.Attr) slog.Attr) *windowsEventLogHandler {
	if el == nil {
		return nil
	}
	return &windowsEventLogHandler{
		el:       el,
		level:    level,
		replacer: replacer,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *windowsEventLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle handles the Record.
func (h *windowsEventLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Build the log message
	var buf strings.Builder

	// Add the message first
	if r.Message != "" {
		buf.WriteString(r.Message)
	}

	// Add attributes
	attrs := make([]slog.Attr, 0, len(h.attrs)+r.NumAttrs())
	attrs = append(attrs, h.attrs...)

	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	// Apply the replacer function to each attribute
	for _, attr := range attrs {
		if h.replacer != nil {
			attr = h.replacer(h.groups, attr)
		}

		// Skip empty attributes
		if attr.Key == "" {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(attr.Key)
		buf.WriteString("=")
		buf.WriteString(attr.Value.String())
	}

	message := buf.String()
	if message == "" {
		return nil // Don't log empty messages
	}

	// Determine the event log level and write to Windows Event Log
	switch r.Level {
	case slog.LevelDebug, slog.LevelInfo:
		return h.el.Info(1, message)
	case slog.LevelWarn:
		return h.el.Warning(1, message)
	case slog.LevelError:
		return h.el.Error(1, message)
	default:
		// For unknown levels, default to Info
		return h.el.Info(1, message)
	}
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
func (h *windowsEventLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	newAttrs = append(newAttrs, h.attrs...)
	newAttrs = append(newAttrs, attrs...)

	newGroups := make([]string, len(h.groups))
	copy(newGroups, h.groups)

	return &windowsEventLogHandler{
		el:       h.el,
		level:    h.level,
		attrs:    newAttrs,
		groups:   newGroups,
		replacer: h.replacer,
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups.
func (h *windowsEventLogHandler) WithGroup(name string) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs))
	copy(newAttrs, h.attrs)

	newGroups := make([]string, 0, len(h.groups)+1)
	newGroups = append(newGroups, h.groups...)
	newGroups = append(newGroups, name)

	return &windowsEventLogHandler{
		el:       h.el,
		level:    h.level,
		attrs:    newAttrs,
		groups:   newGroups,
		replacer: h.replacer,
	}
}

// Close closes the Windows Event Log.
func (h *windowsEventLogHandler) Close() error {
	if h.el != nil {
		return h.el.Close()
	}
	return nil
}
