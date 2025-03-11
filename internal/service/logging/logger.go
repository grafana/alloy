package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/slogadapter"
)

type EnabledAware interface {
	Enabled(context.Context, slog.Level) bool
}

// Logger is the logging subsystem of Alloy. It supports being dynamically
// updated at runtime.
type Logger struct {
	inner io.Writer // Writer passed to New.

	bufferMut    sync.RWMutex
	buffer       []*bufferedItem // Store logs before correctly determine the log format
	hasLogFormat bool            // Confirmation whether log format has been determined

	level        *slog.LevelVar       // Current configured level.
	format       *formatVar           // Current configured format.
	writer       *writerVar           // Current configured multiwriter (inner + write_to).
	handler      *handler             // Handler which handles logs.
	deferredSlog *deferredSlogHandler // This handles deferred logging for slog.
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

	// Apply the options
	l.level.Set(slogLevel(o.Level).Level())
	l.format.Set(o.Format)
	l.writer.SetInnerWriter(w)
	if o.WriteTo != nil {
		l.writer.SetLokiWriter(&lokiWriter{f: o.WriteTo})
	}

	// Mark that we have a log format configured
	l.bufferMut.Lock()
	l.hasLogFormat = true
	l.bufferMut.Unlock()

	return l, nil
}

// NewNop returns a logger that does nothing
func NewNop() *Logger {
	l, _ := NewDeferred(io.Discard)
	return l
}

// NewDeferred creates a new logger with the default log level and format.
// The logger is not updated during initialization.
func NewDeferred(w io.Writer) (*Logger, error) {
	var (
		leveler slog.LevelVar
		format  formatVar
		writer  writerVar
	)
	l := &Logger{
		inner: w,

		buffer:       []*bufferedItem{},
		hasLogFormat: false,

		level:  &leveler,
		format: &format,
		writer: &writer,
		handler: &handler{
			w:         &writer,
			leveler:   &leveler,
			formatter: &format,
			replacer:  replace,
		},
	}
	l.deferredSlog = newDeferredHandler(l)

	return l, nil
}

// Handler returns a [slog.Handler]. The returned Handler remains valid if l is
// updated.
func (l *Logger) Handler() slog.Handler { return l.deferredSlog }

func (l *Logger) SetTemporaryWriter(w io.Writer) {
	l.writer.SetTemporaryWriter(w)
}

func (l *Logger) RemoveTemporaryWriter() {
	l.writer.RemoveTemporaryWriter()
}

// Log implements log.Logger.
func (l *Logger) Log(kvps ...interface{}) error {
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
			Entry: logproto.Entry{
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

	lokiWriter  *lokiWriter
	innerWriter io.Writer
	tmpWriter   io.Writer
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

func (w *writerVar) SetInnerWriter(writer io.Writer) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.innerWriter = writer
}

func (w *writerVar) SetLokiWriter(writer *lokiWriter) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.lokiWriter = writer
}

func (w *writerVar) Write(p []byte) (int, error) {
	w.mut.RLock()
	defer w.mut.RUnlock()

	if w.innerWriter == nil {
		return 0, fmt.Errorf("no writer available")
	}

	// The following is effectively an io.Multiwriter, but without updating
	// the Multiwriter each time tmpWriter is added or removed.
	if _, err := w.innerWriter.Write(p); err != nil {
		return 0, err
	}

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
	kvps    []interface{}
	handler *deferredSlogHandler
	record  slog.Record
}
