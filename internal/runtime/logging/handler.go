package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// We need an implementation of slog.Handler that always matches the current
// configuration of Logger at runtime.
//
// The challenge is that slog.Handler.WithAttrs and slog.Handler.WithGroup are
// expected to return copies. We need our copies to also match the current
// configuration of the Logger at runtime, even after the copies are returned.
//
// We do this by using a pull-based system for how the various handlers are
// expected to behave. Handlers will look up whether they should be logging as
// JSON or logfmt, and create a new inner handler if needed.

type handler struct {
	// w owns every sink: innerWriter (stderr), lokiWriter, tmpWriter,
	// and the optional Windows event log.
	w         *writerVar
	leveler   slog.Leveler
	formatter formatter
	replacer  func(groups []string, a slog.Attr) slog.Attr

	nested []nesting

	// scratchPool holds per-call scratch (a cached slog handler + leveledWriter).
	// It's per handler instance so each scratch's cached handler matches this
	// handler's nested attrs/groups, and pooled (rather than a single shared
	// handler) because the per-call level on the leveledWriter is mutable state
	// that can't be shared across goroutines.
	scratchPool sync.Pool
}

func newHandler(
	w *writerVar,
	leveler slog.Leveler,
	formatter formatter,
	replacer func(groups []string, a slog.Attr) slog.Attr,
	nested []nesting,
) *handler {

	return &handler{
		w:         w,
		leveler:   leveler,
		formatter: formatter,
		replacer:  replacer,
		nested:    nested,
		scratchPool: sync.Pool{
			New: func() any {
				return &scratch{lw: &leveledWriter{w: w}}
			},
		},
	}
}

type scratch struct {
	// Carries the record's level.
	lw *leveledWriter

	// Formats the record and writes the bytes straight to the configured sinks.
	hdlr slog.Handler

	// Records which format the cached handler was built for,
	// so a runtime format change triggers a rebuild.
	format Format
}

// nesting is used since attrs and groups need to be nested in the order they were entered.
type nesting struct {
	attrs []slog.Attr
	group string
}

type formatter interface {
	Format() Format
}

var _ slog.Handler = (*handler)(nil)

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool {
	// Check the underlying leveler directly rather than going through a handler.
	return l >= h.leveler.Level()
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	s := h.getScratch()
	defer h.scratchPool.Put(s)

	s.lw.level = r.Level
	return s.hdlr.Handle(ctx, r)
}

// getScratch returns scratch space whose cached handler matches the current
// format, rebuilding the handler only when the format changed (or on first use).
func (h *handler) getScratch() *scratch {
	expectFormat := h.formatter.Format()
	s := h.scratchPool.Get().(*scratch)
	if s.hdlr == nil || s.format != expectFormat {
		s.hdlr = h.buildHandler(s.lw)
		s.format = expectFormat
	}
	return s
}

// buildHandler constructs a fresh slog handler writing into w, with the
// current format and the WithAttrs/WithGroup chain applied.
func (h *handler) buildHandler(w io.Writer) slog.Handler {
	handlerOpts := slog.HandlerOptions{
		AddSource: false,
		Level:     h.leveler,
		// Replace attributes with how they were represented in go-kit/log
		// for consistency.
		ReplaceAttr: h.replacer,
	}
	var hdlr slog.Handler
	switch h.formatter.Format() {
	case FormatLogfmt:
		hdlr = slog.NewTextHandler(w, &handlerOpts)
	case FormatJSON:
		hdlr = slog.NewJSONHandler(w, &handlerOpts)
	default:
		panic(fmt.Sprintf("unknown format %v", h.formatter.Format()))
	}
	// Replay our groups and attrs in the order they were entered.
	for _, n := range h.nested {
		if n.group != "" {
			hdlr = hdlr.WithGroup(n.group)
		} else {
			hdlr = hdlr.WithAttrs(n.attrs)
		}
	}
	return hdlr
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newNest := make([]nesting, 0, len(h.nested)+1)
	newNest = append(newNest, h.nested...)
	newNest = append(newNest, nesting{
		attrs: attrs,
	})

	return newHandler(h.w, h.leveler, h.formatter, h.replacer, newNest)
}

func (h *handler) WithGroup(name string) slog.Handler {
	newNest := make([]nesting, 0, len(h.nested)+1)
	newNest = append(newNest, h.nested...)
	newNest = append(newNest, nesting{
		group: name,
	})
	return newHandler(h.w, h.leveler, h.formatter, h.replacer, newNest)
}

var unsafeKeyCharReplacer = strings.NewReplacer(
	" ", "_",
	"=", "_",
	"\"", "_",
	"\t", "_",
	"\n", "_",
	"\v", "_",
	"\f", "_",
	"\r", "_",
)

func replace(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}

	switch a.Key {
	case slog.TimeKey:
		return slog.Attr{
			Key:   "ts",
			Value: slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano)),
		}

	case slog.SourceKey:
		source, ok := a.Value.Any().(*slog.Source)
		if !ok {
			// The attribute value doesn't match our expected type. This probably
			// indicates it's from a usage of go-kit/log that happens to also
			// have a field called [slog.SourceKey].
			//
			// Return the attribute unmodified.
			return a
		}

		if source.File == "" && source.Line == 0 {
			// Drop attributes with no source information.
			return slog.Attr{}
		}

		return a

	case slog.MessageKey:
		if a.Value.String() == "" {
			// Drop empty message keys.
			return slog.Attr{}
		}

	case slog.LevelKey:
		level := a.Value.Any().(slog.Level)

		// Override the value names to match go-kit/log, which would otherwise
		// print as all-caps DEBUG/INFO/WARN/ERROR.
		switch level {
		case slog.LevelDebug:
			return slog.Attr{Key: "level", Value: slog.StringValue("debug")}
		case slog.LevelInfo:
			return slog.Attr{Key: "level", Value: slog.StringValue("info")}
		case slog.LevelWarn:
			return slog.Attr{Key: "level", Value: slog.StringValue("warn")}
		case slog.LevelError:
			return slog.Attr{Key: "level", Value: slog.StringValue("error")}
		}
	}

	return slog.Attr{
		Key:   unsafeKeyCharReplacer.Replace(strings.TrimSpace(a.Key)),
		Value: a.Value,
	}
}
