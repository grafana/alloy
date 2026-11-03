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
	// parent is the handler this one was derived from, nil for the root.
	// A child that buildHandlers did not reach builds itself from it.
	parent *deferredSlogHandler
}

func newDeferredHandler(l *Logger) *deferredSlogHandler {
	return &deferredSlogHandler{
		children: make([]*deferredSlogHandler, 0),
		l:        l,
	}
}

func (d *deferredSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if h := d.realHandler(); h != nil {
		return h.Handle(ctx, r)
	}
	d.l.addRecord(r, d)
	return nil
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (d *deferredSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h := d.realHandler(); h != nil {
		return h.Enabled(ctx, level)
	}
	return true
}

// realHandler returns the built handler, or nil while the logging config is
// still pending. A child that buildHandlers did not reach, because it was
// registered while the parent was being built, is built here from its parent
// instead of buffering forever: a record buffered by such a child would be
// replayed by flushBuffer into addRecord, which waits on the lock flushBuffer
// holds.
func (d *deferredSlogHandler) realHandler() slog.Handler {
	d.mut.RLock()
	handle, parent := d.handle, d.parent
	d.mut.RUnlock()
	if handle != nil || parent == nil {
		return handle
	}

	parentHandle := parent.realHandler()
	if parentHandle == nil {
		return nil
	}
	d.mut.Lock()
	defer d.mut.Unlock()
	if d.handle == nil {
		d.handle = d.buildFrom(parentHandle)
	}
	return d.handle
}

// buildFrom derives this node's handler from its parent's built handler.
// The caller holds d.mut.
func (d *deferredSlogHandler) buildFrom(parent slog.Handler) slog.Handler {
	if d.group != "" {
		return parent.WithGroup(d.group)
	}
	return parent.WithAttrs(d.attrs)
}

// WithAttrs returns a new [TextHandler] whose attributes consists
// of h's attributes followed by attrs.
func (d *deferredSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// A write lock: children is appended to, and two concurrent appends under
	// a read lock can drop one of the children, which buildHandlers then never
	// reaches.
	d.mut.Lock()
	defer d.mut.Unlock()

	if d.handle != nil {
		return d.handle.WithAttrs(attrs)
	}

	child := &deferredSlogHandler{
		attrs:    attrs,
		children: make([]*deferredSlogHandler, 0),
		l:        d.l,
		parent:   d,
	}
	d.children = append(d.children, child)
	return child
}

func (d *deferredSlogHandler) WithGroup(name string) slog.Handler {
	d.mut.Lock()
	defer d.mut.Unlock()

	if d.handle != nil {
		return d.handle.WithGroup(name)
	}

	child := &deferredSlogHandler{
		children: make([]*deferredSlogHandler, 0),
		group:    name,
		l:        d.l,
		parent:   d,
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
		d.handle = d.l.handler
	} else {
		d.handle = d.buildFrom(parent)
	}
	for _, child := range d.children {
		child.buildHandlers(d.handle)
	}
	d.children = nil
}
