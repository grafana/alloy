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
	"github.com/grafana/alloy/internal/slogadapter"
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
	// the optional Windows Event Log; Update toggles state on writerVar
	// rather than swapping handlers.
	handler      *handler
	deferredSlog *deferredSlogHandler // Buffers slog output until config is loaded, then delegates to handler.

	eventLogOpener eventlog.EventLogOpener // Opens the Windows event log; set in NewDeferred, overridable in tests.
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
		leveler slog.LevelVar
		format  formatVar
	)
	// innerWriter is stable for the life of the Logger; destinations that
	// want to suppress it (windows_event_log) flip writerVar.suppressInner
	// instead of swapping the writer.
	writer := &writerVar{innerWriter: w}
	bh := &handler{
		w:         writer,
		leveler:   &leveler,
		formatter: &format,
		replacer:  replace,
	}

	l := &Logger{
		buffer:       []*bufferedItem{},
		hasLogFormat: false,

		level:          &leveler,
		format:         &format,
		writer:         writer,
		handler:        bh,
		eventLogOpener: eventlog.GetEventLogOpener(),
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
	if err := l.applyDestination(o.Destination); err != nil {
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

// applyDestination toggles state on l.writer to match the new destination.
// Must be called with l.bufferMut held by the caller.
//
// The architecture is loss-free by construction:
//   - l.handler is stable — Update never reassigns it.
//   - l.writer owns every sink (stderr, write_to, tmp, event log).
//     Transitions are atomic from Dispatch's point of view:
//     SwitchToEventLog and SwitchToInnerOnly each hold the write lock
//     across every state change they make, so a single Dispatch never
//     observes a mid-transition mix of suppressInner and eventLog.
//   - A record may straddle a destination switch — handler.Handle reads
//     FastPathFlags before Dispatch acquires its own RLock, so the two
//     can observe different configurations. Dispatch's fallback handles
//     this: when bytes can't reach the event log (no level supplied or
//     event log detached), they fall through to innerWriter even if
//     suppressInner is true. A switching record may land on stderr
//     instead of the event log, but it is never dropped.
func (l *Logger) applyDestination(d LogDestination) error {
	if d == LogDestinationWindowsEventLog {
		el, err := l.eventLogOpener("Alloy")
		if err != nil {
			return fmt.Errorf("failed to open Windows Event Log: %w", err)
		}
		return l.writer.SwitchToEventLog(el)
	}
	return l.writer.SwitchToInnerOnly()
}

// flushBuffer drains and replays any logs that were buffered before
// Update finished resolving the log format. It must be called AFTER the
// new destination has been applied (so replayed records go through the
// right handler) and AFTER l.deferredSlog.buildHandlers has run (so
// child handlers point at the new l.handler).
//
// Holds bufferMut for the entire replay so concurrent Log() calls block
// until the buffer is drained, preserving the order guarantee that
// buffered logs appear before newly-arriving ones.
func (l *Logger) flushBuffer() {
	l.bufferMut.Lock()
	defer l.bufferMut.Unlock()
	l.hasLogFormat = true
	buffer := l.buffer
	l.buffer = nil

	for _, item := range buffer {
		if len(item.kvps) > 0 {
			slogadapter.GoKit(l.handler).Log(item.kvps...)
		} else if item.handler.Enabled(context.Background(), item.record.Level) {
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

// Log implements log.Logger.
func (l *Logger) Log(kvps ...any) error {
	// Buffer logs before confirming log format is configured in `logging` block.
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

	// NOTE(rfratto): slogadapter is a temporary shim while log/slog is still
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

type writerVar struct {
	mut sync.RWMutex

	lokiWriter    *lokiWriter
	innerWriter   io.Writer
	tmpWriter     io.Writer
	suppressInner bool // when true, Write skips innerWriter

	// eventLog is the optional Windows Event Log sink. When non-nil and the
	// caller of Dispatch supplied a level, Dispatch maps the level to
	// Info/Warning/Error and forwards the formatted bytes. Protected by mut
	// so concurrent Dispatch calls drain before SwitchToInnerOnly /
	// SwitchToEventLog release or replace the underlying OS handle.
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

// SwitchToEventLog atomically installs el as the event log sink and
// flips suppressInner=true. If an event log was already attached, it is
// closed first. The write lock is held across the close + install +
// flip, so no Dispatch ever observes the mid-state where suppressInner
// and eventLog disagree about whether bytes belong on stderr or the
// event log.
//
// Returns the previous event log's Close error, if any. Callers should
// open el fresh — event_log → event_log reloads close + reopen the
// handle, which is cheap (DeregisterEventSource + RegisterEventSource).
func (w *writerVar) SwitchToEventLog(el eventlog.EventLog) error {
	w.mut.Lock()
	defer w.mut.Unlock()
	var closeErr error
	if w.eventLog != nil {
		closeErr = w.eventLog.Close()
	}
	w.eventLog = el
	w.suppressInner = true
	return closeErr
}

// SwitchToInnerOnly atomically flips suppressInner=false and closes any
// attached event log. The write lock is held across both state changes
// and the Close, so Dispatch never observes the mid-state where
// suppressInner=false and eventLog!=nil (which would double-deliver to
// stderr and the event log). Idempotent.
func (w *writerVar) SwitchToInnerOnly() error {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.suppressInner = false
	if w.eventLog == nil {
		return nil
	}
	err := w.eventLog.Close()
	w.eventLog = nil
	return err
}

// FastPathFlags returns whether any sink is active and whether the event
// log sink in particular is attached. Both checks happen under a single
// RLock so callers see a consistent snapshot. Used by handler.Handle
// to decide between the fast path (no buffer; direct slog handler write
// to writerVar) and the slow path (capture bytes + dispatch with level).
//
// io.Discard is treated as "no sink": tests and benchmarks pass it as
// the underlying writer when they want to measure the logger path
// without actually consuming bytes, and the bytes handler should skip
// formatting in that case.
func (w *writerVar) FastPathFlags() (hasSink, hasEventLog bool) {
	w.mut.RLock()
	defer w.mut.RUnlock()
	hasEventLog = w.eventLog != nil
	hasSink = hasEventLog ||
		(w.innerWriter != nil && w.innerWriter != io.Discard && !w.suppressInner) ||
		w.lokiWriter != nil ||
		w.tmpWriter != nil
	return
}

// HasSink reports whether any active sink will accept bytes. Kept for
// callers that don't need to distinguish the event-log case.
func (w *writerVar) HasSink() bool {
	hasSink, _ := w.FastPathFlags()
	return hasSink
}

// Dispatch is the canonical write entry point. It fans p out to every
// active byte-stream sink (innerWriter, lokiWriter, tmpWriter). If
// eventLogLevel is non-nil and an event log is attached, p is also
// forwarded to the event log with the level mapped to Info/Warning/Error.
//
// Suppression of innerWriter only applies when the bytes can actually
// reach the event log. If they can't — because the caller didn't supply
// a level (fast path) or because the event log was detached by a
// concurrent reload — innerWriter receives the bytes regardless of
// w.suppressInner. This is the loss-prevention property: during a
// destination switch, a record may land on stderr instead of the event
// log, but it is never dropped.
//
// The trailing newline added by slog's TextHandler/JSONHandler is trimmed
// before handing the message to the event log API, which renders cleaner
// in Event Viewer.
//
// A nil eventLogLevel is the only way to skip the event log explicitly —
// passing slog.Level(0) would dispatch as Info, because zero is a valid
// slog.Level.
func (w *writerVar) Dispatch(p []byte, eventLogLevel *slog.Level) error {
	w.mut.RLock()
	defer w.mut.RUnlock()

	canReachEventLog := eventLogLevel != nil && w.eventLog != nil
	writeInner := w.innerWriter != nil && (!w.suppressInner || !canReachEventLog)
	if writeInner {
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
	if !canReachEventLog {
		return nil
	}
	msg := p
	if n := len(msg); n > 0 && msg[n-1] == '\n' {
		msg = msg[:n-1]
	}
	switch *eventLogLevel {
	case slog.LevelWarn:
		return w.eventLog.Warning(1, string(msg))
	case slog.LevelError:
		return w.eventLog.Error(1, string(msg))
	default:
		return w.eventLog.Info(1, string(msg))
	}
}

// Write is the io.Writer adapter used by the fast-path slog handler.
// Equivalent to Dispatch(p, nil) — byte-stream sinks only, no event log.
// For the event-log path use Dispatch directly with a non-nil level.
func (w *writerVar) Write(p []byte) (int, error) {
	if err := w.Dispatch(p, nil); err != nil {
		return 0, err
	}
	return len(p), nil
}

type bufferedItem struct {
	kvps    []any
	handler *deferredSlogHandler
	record  slog.Record
}
