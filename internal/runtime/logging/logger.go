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
	"github.com/grafana/alloy/internal/runtime/logging/eventlog"
	"github.com/grafana/loki/pkg/push"
)

type EnabledAware interface {
	Enabled(context.Context, slog.Level) bool
}

// Logger is the logging subsystem of Alloy. It supports being dynamically
// updated at runtime.
type Logger struct {
	bufferMut    sync.RWMutex
	buffer       []*bufferedItem // Store logs before correctly determine the log format
	hasLogFormat bool            // Confirmation whether log format has been determined

	level  *slog.LevelVar // Current configured level.
	format *formatVar     // Current configured format.
	writer *writerVar     // Current configured multiwriter (inner + write_to + event log).

	// handler is the single slog.Handler dispatched to by both the
	// gokit and slog paths. It is set once in NewDeferred and never
	// reassigned. Its writer (a *writerVar) owns every sink, including
	// the optional Windows Event Log.
	handler      *handler
	deferredSlog *deferredSlogHandler // Buffers slog output until config is loaded, then delegates to handler.
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
	return slog.New(slog.DiscardHandler)
}

// NewDeferred creates a new logger with the default log level and format.
// The logger is not updated during initialization.
func NewDeferred(w io.Writer) (*Logger, error) {
	var (
		leveler slog.LevelVar
		format  formatVar
	)
	// innerWriter is stable for the life of the Logger; destinations that
	// want to suppress it (windows_event_log) lazily open an event log via
	// the writer's own opener instead of swapping the writer.
	writer := &writerVar{
		innerWriter:    w,
		eventLogOpener: eventlog.GetEventLogOpener(),
	}
	bh := newHandler(writer, &leveler, &format, replace, nil)

	l := &Logger{
		buffer:       []*bufferedItem{},
		hasLogFormat: false,

		level:   &leveler,
		format:  &format,
		writer:  writer,
		handler: bh,
	}
	l.deferredSlog = newDeferredHandler(l)

	return l, nil
}

// Handler returns a [slog.Handler]. The returned Handler remains valid if l is
// updated.
func (l *Logger) Handler() slog.Handler { return l.deferredSlog }

// Slog returns a [slog.Logger]. The returned logger remains valid if l is
// updated.
func (l *Logger) Slog() *slog.Logger { return slog.New(l.deferredSlog) }

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
	if err := l.writer.SetDestination(o.Destination); err != nil {
		l.bufferMut.Unlock()
		return err
	}
	l.writer.SetLokiWriter(o.WriteTo)
	l.bufferMut.Unlock()

	// Rebuild deferred slog handlers outside bufferMut to avoid a deadlock
	// with concurrent Handle() calls (they hold a child handler's RLock
	// while waiting for bufferMut via addRecord).
	if l.deferredSlog != nil {
		l.deferredSlog.buildHandlers(nil)
	}
	l.flushBuffer()
	return nil
}

// flushBuffer drains and replays any logs that were buffered before
// Update finished resolving the log format. It must be called AFTER the
// new destination has been applied (so replayed records go through the
// right handler) and AFTER l.deferredSlog.buildHandlers has run (so
// child handlers point at the new l.handler).
//
// Holds bufferMut for the entire replay so concurrent log calls calls block
// until the buffer is drained, preserving the order guarantee that
// buffered logs appear before newly-arriving ones.
func (l *Logger) flushBuffer() {
	l.bufferMut.Lock()
	defer l.bufferMut.Unlock()
	l.hasLogFormat = true
	buffer := l.buffer
	l.buffer = nil

	for _, item := range buffer {
		if item.handler.Enabled(context.Background(), item.record.Level) {
			_ = item.handler.Handle(context.Background(), item.record)
		}
	}
}

func (l *Logger) SetTemporaryWriter(w io.Writer) {
	l.writer.SetTemporaryWriter(w)
}

func (l *Logger) RemoveTemporaryWriter() {
	l.writer.RemoveTemporaryWriter()
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

type writerVar struct {
	mut sync.RWMutex

	// Inner writer is stderr on the production path.
	// Tests may set it to something else.
	innerWriter io.Writer

	lokiWriter *lokiWriter
	tmpWriter  io.Writer

	// eventLogOpener lazily creates the Windows Event Log handle the first
	// time SetDestination enters event-log mode. Set in NewDeferred,
	// overridable in tests.
	eventLogOpener eventlog.EventLogOpener

	// eventLog is the single switch between the two primary-sink modes:
	// when nil, Dispatch writes to innerWriter (stderr); when non-nil,
	// innerWriter is suppressed and Dispatch routes to the event log
	// instead. lokiWriter and tmpWriter are always written to regardless.
	eventLog eventlog.EventLog
}

func (w *writerVar) SetTemporaryWriter(writer io.Writer) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.tmpWriter = writer
}

func (w *writerVar) RemoveTemporaryWriter() {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.tmpWriter = nil
}

func (w *writerVar) SetLokiWriter(receivers []loki.LogsReceiver) {
	w.mut.Lock()
	defer w.mut.Unlock()
	if len(receivers) > 0 {
		w.lokiWriter = &lokiWriter{receivers}
	} else {
		w.lokiWriter = nil
	}
}

func (w *writerVar) SetDestination(d LogDestination) error {
	w.mut.Lock()
	defer w.mut.Unlock()

	if d == LogDestinationWindowsEventLog {
		if w.eventLog != nil {
			return nil // already open; reuse the handle
		}
		el, err := w.eventLogOpener("Alloy")
		if err != nil {
			return fmt.Errorf("failed to open Windows Event Log: %w", err)
		}
		w.eventLog = el
		return nil
	}

	if w.eventLog == nil {
		return nil
	}
	err := w.eventLog.Close()
	w.eventLog = nil
	return err
}

// Dispatch is the canonical write entry point. It fans p out to every
// active sink: innerWriter (unless suppressed), lokiWriter, tmpWriter,
// and, when attached, the event log.
//
// SetDestination holds the write lock across the swap it makes, so any
// Dispatch RLock observes one of two states:
// 1. eventLog=nil — stderr mode.
// 2. eventLog!=nil — event_log mode.
func (w *writerVar) Dispatch(p []byte, eventLogLevel *slog.Level) error {
	w.mut.RLock()
	defer w.mut.RUnlock()

	if w.eventLog == nil && w.innerWriter != nil {
		if _, err := w.innerWriter.Write(p); err != nil {
			return err
		}
	}
	if w.lokiWriter != nil {
		if _, err := w.lokiWriter.Write(p); err != nil {
			return err
		}
	}
	if w.tmpWriter != nil {
		if _, err := w.tmpWriter.Write(p); err != nil {
			return err
		}
	}
	if w.eventLog == nil {
		return nil
	}

	// The trailing newline added by slog's TextHandler/JSONHandler is trimmed
	// before handing the message to the event log API, which renders cleaner
	// in Event Viewer.
	msg := p
	if n := len(msg); n > 0 && msg[n-1] == '\n' {
		msg = msg[:n-1]
	}

	// Loss-prevention property: in event_log mode, a fast-path Write call
	// (eventLogLevel=nil) can still occur if a destination switch happened
	// between handler.Handle's FastPathFlags read and the slog handler's
	// Write. Rather than dropping the record or misrouting it to stderr,
	// Dispatch defaults to Info and delivers it to the event log — the
	// operator's configured destination. The level may be slightly off
	// (the original record's level is embedded in the formatted text), but
	// the message reaches Event Viewer.
	if eventLogLevel != nil {
		switch *eventLogLevel {
		case slog.LevelWarn:
			return w.eventLog.Warning(1, string(msg))
		case slog.LevelError:
			return w.eventLog.Error(1, string(msg))
		}
	}
	return w.eventLog.Info(1, string(msg))
}

// Write is the io.Writer adapter used by the fast-path slog handler.
// Equivalent to Dispatch(p, nil); see Dispatch for how a fast-path
// write is handled when the event log is attached.
func (w *writerVar) Write(p []byte) (int, error) {
	if err := w.Dispatch(p, nil); err != nil {
		return 0, err
	}
	return len(p), nil
}

// leveledWriter is an io.Writer that remembers the slog level of the record
// currently being formatted, so writerVar.Dispatch can route it to the event
// log's Info/Warning/Error API. It is the event-log counterpart of
// writerVar.Write, which carries no level (fast/stderr path).
type leveledWriter struct {
	w     *writerVar
	level slog.Level
}

func (lw *leveledWriter) Write(p []byte) (int, error) {
	if err := lw.w.Dispatch(p, &lw.level); err != nil {
		return 0, err
	}
	return len(p), nil
}

type bufferedItem struct {
	handler *deferredSlogHandler
	record  slog.Record
}
