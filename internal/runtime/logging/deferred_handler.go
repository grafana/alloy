package logging

import (
	"context"
	"log/slog"
	"sync"
)

// deferredSlogHandler is used if you are using a slog handler before the logging config block is processed.
type deferredSlogHandler struct {
	mut      sync.RWMutex
	group    string
	attrs    []slog.Attr
	children []*deferredSlogHandler
	handle   slog.Handler
	l        *Logger
}

func newDeferredHandler(l *Logger) *deferredSlogHandler {
	return &deferredSlogHandler{
		children: make([]*deferredSlogHandler, 0),
		l:        l,
	}
}

func (d *deferredSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	d.mut.RLock()
	defer d.mut.RUnlock()

	if d.handle != nil {
		return d.handle.Handle(ctx, r)
	}
	d.l.addRecord(r, d)
	return nil
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (d *deferredSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	d.mut.RLock()
	defer d.mut.RUnlock()

	if d.handle != nil {
		return d.handle.Enabled(ctx, level)
	}
	return true
}

// WithAttrs returns a new [TextHandler] whose attributes consists
// of h's attributes followed by attrs.
func (d *deferredSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	d.mut.RLock()
	defer d.mut.RUnlock()

	if d.handle != nil {
		return d.handle.WithAttrs(attrs)
	}

	child := &deferredSlogHandler{
		attrs:    attrs,
		children: make([]*deferredSlogHandler, 0),
		l:        d.l,
	}
	d.children = append(d.children, child)
	return child
}

func (d *deferredSlogHandler) WithGroup(name string) slog.Handler {
	d.mut.RLock()
	defer d.mut.RUnlock()

	if d.handle != nil {
		return d.handle.WithGroup(name)
	}

	child := &deferredSlogHandler{
		children: make([]*deferredSlogHandler, 0),
		group:    name,
		l:        d.l,
	}
	d.children = append(d.children, child)
	return child
}

// buildHandlers will recursively build actual handlers, this should only be called before replaying once the logging config
// block is set.
func (d *deferredSlogHandler) buildHandlers(parent slog.Handler) {
	d.mut.Lock()
	defer d.mut.Unlock()

	// Root node will not have attrs or groups.
	if parent == nil {
		if d.l.windowsEventLogHandler != nil {
			d.handle = d.l.windowsEventLogHandler
		} else {
			d.handle = d.l.handler
		}
	} else {
		if d.group != "" {
			d.handle = parent.WithGroup(d.group)
		} else {
			d.handle = parent.WithAttrs(d.attrs)
		}
	}
	for _, child := range d.children {
		child.buildHandlers(d.handle)
	}
	d.children = nil
}
