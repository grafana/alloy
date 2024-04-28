package logging

import (
	"context"
	"log/slog"
	"sync"
)

type DeferredHandler struct {
	mut      sync.Mutex
	group    string
	attrs    []slog.Attr
	children []*DeferredHandler
	parent   *DeferredHandler
	handle   slog.Handler
	l        *Logger
}

func NewDeferredHandler(l *Logger) *DeferredHandler {
	return &DeferredHandler{
		children: make([]*DeferredHandler, 0),
		l:        l,
	}
}

func (d *DeferredHandler) Handle(_ context.Context, r slog.Record) error {
	d.l.addRecord(r, d)
	return nil
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *DeferredHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

// WithAttrs returns a new [TextHandler] whose attributes consists
// of h's attributes followed by attrs.
func (h *DeferredHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mut.Lock()
	defer h.mut.Unlock()

	child := &DeferredHandler{
		attrs:    attrs,
		children: make([]*DeferredHandler, 0),
		l:        h.l,
		parent:   h,
	}
	h.children = append(h.children, child)
	return child
}

func (h *DeferredHandler) WithGroup(name string) slog.Handler {
	h.mut.Lock()
	defer h.mut.Unlock()

	child := &DeferredHandler{
		children: make([]*DeferredHandler, 0),
		group:    name,
		l:        h.l,
		parent:   h,
	}
	h.children = append(h.children, child)
	return child
}

// buildHandlers will recursively build actual handlers, this should only be called before replaying.
func (h *DeferredHandler) buildHandlers(parent slog.Handler) {
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
