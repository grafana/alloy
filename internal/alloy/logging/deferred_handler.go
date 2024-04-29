package logging

import (
	"context"
	"log/slog"
	"sync"
)

// deferredSlogHandler is used if you are using a slog handler before the logging config block is processed.
type deferredSlogHandler struct {
	mut      sync.Mutex
	group    string
	attrs    []slog.Attr
	children []*deferredSlogHandler
	parent   *deferredSlogHandler
	handle   slog.Handler
	l        *Logger
}

func newDeferredHandler(l *Logger) *deferredSlogHandler {
	return &deferredSlogHandler{
		children: make([]*deferredSlogHandler, 0),
		l:        l,
	}
}

func (d *deferredSlogHandler) Handle(_ context.Context, r slog.Record) error {
	d.l.addRecord(r, d)
	return nil
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *deferredSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

// WithAttrs returns a new [TextHandler] whose attributes consists
// of h's attributes followed by attrs.
func (h *deferredSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mut.Lock()
	defer h.mut.Unlock()

	child := &deferredSlogHandler{
		attrs:    attrs,
		children: make([]*deferredSlogHandler, 0),
		l:        h.l,
		parent:   h,
	}
	h.children = append(h.children, child)
	return child
}

func (h *deferredSlogHandler) WithGroup(name string) slog.Handler {
	h.mut.Lock()
	defer h.mut.Unlock()

	child := &deferredSlogHandler{
		children: make([]*deferredSlogHandler, 0),
		group:    name,
		l:        h.l,
		parent:   h,
	}
	h.children = append(h.children, child)
	return child
}

// buildHandlers will recursively build actual handlers, this should only be called before replaying once the logging config
// block is set.
func (h *deferredSlogHandler) buildHandlers(parent slog.Handler) {
	// Root node will not have attrs or groups.
	if parent == nil {
		h.handle = h.l.handler
	} else {
		if h.group != "" {
			h.handle = parent.WithGroup(h.group)
		} else {
			h.handle = parent.WithAttrs(h.attrs)
		}
	}
	for _, child := range h.children {
		child.buildHandlers(h.handle)
	}
}
