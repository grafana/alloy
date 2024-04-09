package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
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
	w         io.Writer
	leveler   slog.Leveler
	formatter formatter

	attrs []slog.Attr
	group []string

	mut           sync.RWMutex
	currentFormat Format
	inner         slog.Handler
}

type formatter interface {
	Format() Format
}

var _ slog.Handler = (*handler)(nil)

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool {
	// Bypass the cache and check the underlying leveler directly.
	return l >= h.leveler.Level()
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	return h.buildHandler().Handle(ctx, r)
}

func (h *handler) buildHandler() slog.Handler {
	// Get the expected format for the duration of this call. It's possible that
	// this will be stale by the time the call returns, but it will be correct on
	// the next call.
	expectFormat := h.formatter.Format()

	// Fast path: if our cached handler is still valid, immediately return it.
	h.mut.RLock()
	if h.currentFormat == expectFormat && h.inner != nil {
		defer h.mut.RUnlock()
		return h.inner
	}
	h.mut.RUnlock()

	// Slow path: we need to build a new handler.
	h.mut.Lock()
	defer h.mut.Unlock()

	var newHandler slog.Handler

	handlerOpts := slog.HandlerOptions{
		AddSource: true,
		Level:     h.leveler,

		// Replace attributes with how they were represented in go-kit/log for
		// consistency.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
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

			return a
		},
	}

	switch expectFormat {
	case FormatLogfmt:
		newHandler = slog.NewTextHandler(h.w, &handlerOpts)
	case FormatJSON:
		newHandler = slog.NewJSONHandler(h.w, &handlerOpts)
	default:
		panic(fmt.Sprintf("unknown format %v", expectFormat))
	}

	newHandler = newHandler.WithAttrs(h.attrs)

	h.currentFormat = expectFormat
	h.inner = newHandler
	return newHandler
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := slices.Clone(h.attrs)

	if len(h.group) > 0 {
		// Deeply nest the attributes under the appropriate group.
		groupAttr := slog.Attr{
			Key:   h.group[len(h.group)-1],
			Value: slog.GroupValue(attrs...),
		}
		rem := h.group[:len(h.group)-1]

		for len(rem) > 0 {
			groupAttr = slog.Attr{
				Key:   rem[len(rem)-1],
				Value: slog.GroupValue(groupAttr),
			}
			rem = rem[:len(rem)-1]
		}

		newAttrs = append(newAttrs, groupAttr)
	} else {
		newAttrs = append(newAttrs, attrs...)
	}

	return &handler{
		w:         h.w,
		leveler:   h.leveler,
		formatter: h.formatter,

		attrs: newAttrs,
		group: h.group,
	}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{
		w:         h.w,
		leveler:   h.leveler,
		formatter: h.formatter,

		attrs: h.attrs,
		group: append(slices.Clone(h.group), name),
	}
}
