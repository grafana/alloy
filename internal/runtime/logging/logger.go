package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/slogadapter"
	"github.com/grafana/loki/pkg/push"
)

type EnabledAware interface {
	Enabled(context.Context, slog.Level) bool
}

// Logger is the logging subsystem of Alloy. It supports being dynamically
// updated at runtime.
type Logger struct {
	inner io.Writer // Writer passed to New (typically os.Stderr).

	bufferMut    sync.RWMutex
	buffer       []*bufferedItem // Store logs before correctly determine the log format
	hasLogFormat bool            // Confirmation whether log format has been determined

	level  *slog.LevelVar // Current configured level.
	format *formatVar     // Current configured format.

	// primary writes the primary log output to l.inner (stderr).
	// aux writes the auxiliary fan-out (write_to + tmp) to auxWriter.
	// Both are constructed in NewDeferred and not replaced afterward, so
	// l.handler can be read concurrently without a lock.
	primary *handler
	aux     *handler

	// handler is the unified dispatch point: a multiSlogHandler over
	// {primary, aux}. Set once in NewDeferred and never reassigned.
	handler slog.Handler

	// auxWriter holds the dynamic write_to (loki) and temporary writer (used
	// by the support bundle endpoint) for the aux handler. These are
	// independent of the primary destination, so they receive logs whichever
	// primary is in use.
	auxWriter *auxWriterVar

	// deferredSlog buffers slog records until Update establishes the
	// destination, then delegates to handler. Stable across Updates so
	// callers holding a slog.Handler reference don't need to re-acquire it.
	deferredSlog *deferredSlogHandler
}

var _ EnabledAware = (*Logger)(nil)

// Enabled implements EnabledAware interface.
func (l *Logger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.handler.Enabled(ctx, level)
}

// New creates a New logger with the default log level and format.
func New(w io.Writer, o Options) (*Logger, error) {
	l, err := NewDeferred(w)
	if err != nil {
		return nil, err
	}
	if err = l.Update(o); err != nil {
		return nil, err
	}

	return l, nil
}

// NewNop returns a logger that does nothing
func NewNop() *Logger {
	l, _ := NewDeferred(io.Discard)
	return l
}

// NewSlogNop returns a slog logger backed by a handler that never logs.
func NewSlogNop() *slog.Logger {
	return slog.New(nopSlogHandler{})
}

// NewDeferred creates a new logger with the default log level and format.
// The logger is not updated during initialization.
func NewDeferred(w io.Writer) (*Logger, error) {
	var (
		leveler   slog.LevelVar
		format    formatVar
		auxWriter auxWriterVar
	)
	l := &Logger{
		inner: w,

		buffer:       []*bufferedItem{},
		hasLogFormat: false,

		level:     &leveler,
		format:    &format,
		auxWriter: &auxWriter,
	}
	// Construct primary + aux handlers and the unified dispatch once. They
	// are not replaced afterward; level/format/write_to changes flow through
	// shared variables, and l.handler is read by Log without a lock.
	l.primary = &handler{
		w:         l.inner,
		leveler:   l.level,
		formatter: l.format,
		replacer:  replace,
	}
	l.aux = &handler{
		w:         l.auxWriter,
		leveler:   l.level,
		formatter: l.format,
		replacer:  replace,
	}
	l.handler = multiSlogHandler{handlers: []slog.Handler{l.primary, l.aux}}
	l.deferredSlog = newDeferredHandler(l)

	return l, nil
}

// Handler returns a [slog.Handler]. The returned Handler remains valid if l is
// updated.
func (l *Logger) Handler() slog.Handler { return l.deferredSlog }

// Slog returns a [slog.Logger]. The returned logger remains valid if l is
// updated.
func (l *Logger) Slog() *slog.Logger { return slog.New(l.deferredSlog) }

type nopSlogHandler struct{}

func (nopSlogHandler) Enabled(context.Context, slog.Level) bool { return false }

func (nopSlogHandler) Handle(context.Context, slog.Record) error { return nil }

func (nopSlogHandler) WithAttrs([]slog.Attr) slog.Handler { return nopSlogHandler{} }

func (nopSlogHandler) WithGroup(string) slog.Handler { return nopSlogHandler{} }

// Update re-configures the options used for the logger.
func (l *Logger) Update(o Options) error {
	switch o.Format {
	case FormatLogfmt, FormatJSON:
	default:
		return fmt.Errorf("unrecognized log format %q", o.Format)
	}

	l.bufferMut.Lock()
	l.level.Set(slogLevel(o.Level).Level())
	l.format.Set(o.Format)

	// Configure aux fan-out outputs. Always active, independent of destination.
	if len(o.WriteTo) > 0 {
		l.auxWriter.SetLokiWriter(&lokiWriter{o.WriteTo})
	} else {
		l.auxWriter.SetLokiWriter(nil)
	}
	l.bufferMut.Unlock()

	// Build deferred handlers outside bufferMut to avoid a deadlock: concurrent
	// Handle() calls hold a child handler's RLock while waiting for bufferMut
	// (via addRecord), while Update holding bufferMut and waiting for the child's
	// write lock in buildHandlers creates a cycle.
	if l.deferredSlog != nil {
		l.deferredSlog.buildHandlers(nil)
	}

	// Flip hasLogFormat and drain/replay while holding bufferMut so new Log()
	// calls block on RLock until replay finishes — preserving the original
	// guarantee that buffered logs are emitted before newly-arriving ones.
	l.bufferMut.Lock()
	defer l.bufferMut.Unlock()
	l.hasLogFormat = true
	buffer := l.buffer
	l.buffer = nil

	// Replay buffered logs through the unified handler, which fans out to
	// primary + aux.
	for _, item := range buffer {
		if len(item.kvps) > 0 {
			_ = slogadapter.GoKit(l.handler).Log(item.kvps...)
		} else if item.handler != nil {
			// item.handler is a deferredSlogHandler whose handle has just been
			// rebuilt by buildHandlers above.
			if item.handler.Enabled(context.Background(), item.record.Level) {
				_ = item.handler.Handle(context.Background(), item.record)
			}
		}
	}

	return nil
}

func (l *Logger) SetTemporaryWriter(w io.Writer) {
	l.auxWriter.SetTemporaryWriter(w)
}

func (l *Logger) RemoveTemporaryWriter() {
	l.auxWriter.RemoveTemporaryWriter()
}

// Log implements log.Logger.
func (l *Logger) Log(kvps ...any) error {
	// Buffer logs before confirming log format is configured in `logging` block
	l.bufferMut.RLock()
	if !l.hasLogFormat {
		l.bufferMut.RUnlock()
		l.bufferMut.Lock()
		// Check hasLogFormat again; could have changed since the unlock.
		if !l.hasLogFormat {
			l.buffer = append(l.buffer, &bufferedItem{kvps: kvps})
			l.bufferMut.Unlock()
			return nil
		}
		l.bufferMut.Unlock()
	} else {
		l.bufferMut.RUnlock()
	}

	// NOTE(rfratto): this method is a temporary shim while log/slog is still
	// being adopted throughout the codebase.
	return slogadapter.GoKit(l.handler).Log(kvps...)
}

func (l *Logger) addRecord(r slog.Record, df *deferredSlogHandler) {
	l.bufferMut.Lock()
	defer l.bufferMut.Unlock()

	l.buffer = append(l.buffer, &bufferedItem{
		record:  r,
		handler: df,
	})
}

type lokiWriter struct {
	f []loki.LogsReceiver
}

func (fw *lokiWriter) Write(p []byte) (int, error) {
	for _, receiver := range fw.f {
		// We may have been given a nil value in rare circumstances due to
		// misconfiguration or a component which generates exports after
		// construction.
		//
		// Ignore nil values so we don't panic.
		if receiver == nil {
			continue
		}

		select {
		case receiver.Chan() <- loki.Entry{
			Labels: model.LabelSet{"component": "alloy"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      string(p),
			},
		}:
		default:
			return 0, fmt.Errorf("lokiWriter failed to forward entry, channel was blocked")
		}
	}
	return len(p), nil
}

type formatVar struct {
	mut sync.RWMutex
	f   Format
}

func (f *formatVar) Format() Format {
	f.mut.RLock()
	defer f.mut.RUnlock()
	return f.f
}

func (f *formatVar) Set(format Format) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.f = format
}

// auxWriterVar is the io.Writer used by the aux handler. It fans out writes to
// an optional loki writer (write_to receivers) and an optional temporary
// writer (used by the support bundle endpoint at GET /-/support to capture
// per-request logs into the bundle archive).
type auxWriterVar struct {
	mut sync.RWMutex

	lokiWriter *lokiWriter
	tmpWriter  io.Writer
}

func (w *auxWriterVar) SetTemporaryWriter(writer io.Writer) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.tmpWriter = writer
}

func (w *auxWriterVar) RemoveTemporaryWriter() {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.tmpWriter = nil
}

func (w *auxWriterVar) SetLokiWriter(writer *lokiWriter) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.lokiWriter = writer
}

func (w *auxWriterVar) Write(p []byte) (int, error) {
	w.mut.RLock()
	defer w.mut.RUnlock()

	if w.lokiWriter != nil {
		if _, err := w.lokiWriter.Write(p); err != nil {
			return 0, err
		}
	}

	if w.tmpWriter != nil {
		if _, err := w.tmpWriter.Write(p); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

type bufferedItem struct {
	kvps    []any
	handler *deferredSlogHandler
	record  slog.Record
}
