package logging

import (
	"bytes"
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
	// w owns every sink (innerWriter, lokiWriter, tmpWriter, and the
	// optional event log). It also implements io.Writer so it can be
	// passed straight into slog.NewTextHandler/JSONHandler on the fast
	// path. The slow path (event log attached) uses *writerVar methods
	// directly — Write + WriteRecord + FastPathFlags.
	w         *writerVar
	leveler   slog.Leveler
	formatter formatter

	nested []nesting

	mut           sync.RWMutex
	currentFormat Format
	inner         slog.Handler
	replacer      func(groups []string, a slog.Attr) slog.Attr
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
	// Bypass the cache and check the underlying leveler directly.
	return l >= h.leveler.Level()
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	hasSink, hasEventLog := h.w.FastPathFlags()
	if !hasSink {
		// Skip formatting entirely when no sink is listening — the slog
		// text/JSON handler would otherwise allocate, run ReplaceAttr on
		// every attribute, and hand the bytes to io.Discard.
		return nil
	}
	if !hasEventLog {
		// Stderr happy path: the cached slog handler writes directly into
		// writerVar via the io.Writer interface, no buffer, no per-call
		// rebuild.
		return h.buildHandler().Handle(ctx, r)
	}
	// Event log attached: format once into a buffer so we can hand both
	// the bytes and the record level to writerVar.WriteRecord.
	return h.handleWithEventLog(ctx, r)
}

// handleWithEventLog formats the record into a per-call buffer using a
// freshly-built slog handler, then dispatches via writerVar.WriteRecord
// which knows about the event log and the record level. Only used when
// the event log destination is active.
func (h *handler) handleWithEventLog(ctx context.Context, r slog.Record) error {
	// Cheap level filter before we allocate the buffer and build the
	// per-call slog handler. The inner slog handler filters too, but doing
	// it here also skips the event-log dispatch (which would otherwise
	// receive an empty message).
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	buf := bytesPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bytesPool.Put(buf)

	if err := h.newSlogHandler(buf).Handle(ctx, r); err != nil {
		return err
	}
	if buf.Len() == 0 {
		return nil // belt-and-suspenders for handlers that drop records silently
	}
	return h.w.Dispatch(buf.Bytes(), &r.Level)
}

// bytesPool reuses bytes.Buffer instances across event-log slow-path
// calls to avoid an allocation per record.
var bytesPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// newSlogHandler constructs a fresh slog handler writing into w, with the
// current format and the WithAttrs/WithGroup chain applied. Used by both
// buildHandler (which caches the result for the fast path) and the
// event-log slow path (which can't cache because the writer is per-call).
func (h *handler) newSlogHandler(w io.Writer) slog.Handler {
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

// buildHandler returns a cached slog handler bound to h.w. Used by the
// fast path (no event log) so each Log call avoids the cost of
// reconstructing the WithAttrs/WithGroup chain.
func (h *handler) buildHandler() slog.Handler {
	// Get the expected format for the duration of this call. It's possible
	// that this will be stale by the time the call returns, but it will be
	// correct on the next call.
	expectFormat := h.formatter.Format()

	// Fast path: if our cached handler is still valid, immediately return it.
	h.mut.RLock()
	if h.currentFormat == expectFormat && h.inner != nil {
		defer h.mut.RUnlock()
		return h.inner
	}
	h.mut.RUnlock()

	// Slow path: build a new handler and cache it.
	h.mut.Lock()
	defer h.mut.Unlock()
	h.inner = h.newSlogHandler(h.w)
	h.currentFormat = expectFormat
	return h.inner
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newNest := make([]nesting, 0, len(h.nested)+1)
	newNest = append(newNest, h.nested...)
	newNest = append(newNest, nesting{
		attrs: attrs,
	})

	return &handler{
		w:         h.w,
		leveler:   h.leveler,
		formatter: h.formatter,

		nested:   newNest,
		replacer: h.replacer,
	}
}

func (h *handler) WithGroup(name string) slog.Handler {
	newNest := make([]nesting, 0, len(h.nested)+1)
	newNest = append(newNest, h.nested...)
	newNest = append(newNest, nesting{
		group: name,
	})
	return &handler{
		w:         h.w,
		leveler:   h.leveler,
		formatter: h.formatter,

		nested:   newNest,
		replacer: h.replacer,
	}
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
