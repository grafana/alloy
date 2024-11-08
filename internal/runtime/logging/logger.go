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
	if err = l.Update(o); err != nil {
		return nil, err
	}

	return l, nil
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

// Update re-configures the options used for the logger.
func (l *Logger) Update(o Options) error {
	l.bufferMut.Lock()
	defer l.bufferMut.Unlock()

	switch o.Format {
	case FormatLogfmt, FormatJSON:
		l.hasLogFormat = true
	default:
		return fmt.Errorf("unrecognized log format %q", o.Format)
	}

	l.level.Set(slogLevel(o.Level).Level())
	l.format.Set(o.Format)

	newWriter := l.inner
	newWriter = io.MultiWriter(newWriter, selfLokiWriter)
	if len(o.WriteTo) > 0 {
		newWriter = io.MultiWriter(newWriter, &lokiWriter{o.WriteTo})
	}
	l.writer.Set(newWriter)

	// Build all our deferred handlers
	if l.deferredSlog != nil {
		l.deferredSlog.buildHandlers(nil)
	}
	// Print out the buffered logs since we determined the log format already
	for _, bufferedLogChunk := range l.buffer {
		if len(bufferedLogChunk.kvps) > 0 {
			// the buffered logs are currently only sent to the standard output
			// because the components with the receivers are not running yet
			slogadapter.GoKit(l.handler).Log(bufferedLogChunk.kvps...)
		} else {
			// We now can check to see if if our buffered log is at the right level.
			if bufferedLogChunk.handler.Enabled(context.Background(), bufferedLogChunk.record.Level) {
				// These will always be valid due to the build handlers call above.
				bufferedLogChunk.handler.Handle(context.Background(), bufferedLogChunk.record)
			}
		}
	}
	l.buffer = nil

	return nil
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
	w   io.Writer
}

func (w *writerVar) Set(inner io.Writer) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.w = inner
}

func (w *writerVar) Write(p []byte) (n int, err error) {
	w.mut.RLock()
	defer w.mut.RUnlock()

	if w.w == nil {
		return 0, fmt.Errorf("no writer available")
	}

	return w.w.Write(p)
}

type bufferedItem struct {
	kvps    []interface{}
	handler *deferredSlogHandler
	record  slog.Record
}

// writer to support loki.source.self
// if no components register receivers, this will be a no-op writer
type selfWriter struct {
	lokiWriter         *lokiWriter
	writersByComponent map[string][]loki.LogsReceiver
	mut                sync.Mutex
}

func (sw *selfWriter) Write(p []byte) (int, error) {
	sw.mut.Lock()
	lw := sw.lokiWriter
	sw.mut.Unlock()
	if lw != nil {
		return lw.Write(p)
	}
	return len(p), nil
}
func (sw *selfWriter) register(id string, f []loki.LogsReceiver) {
	sw.mut.Lock()
	defer sw.mut.Unlock()
	sw.writersByComponent[id] = f
	sw.rebuild()
}
func (sw *selfWriter) unregister(id string) {
	sw.mut.Lock()
	defer sw.mut.Unlock()
	delete(sw.writersByComponent, id)
	sw.rebuild()
}

func (sw *selfWriter) rebuild() {
	f := []loki.LogsReceiver{}
	for _, rs := range sw.writersByComponent {
		f = append(f, rs...)
	}
	sw.lokiWriter = &lokiWriter{f: f}
}

var selfLokiWriter = &selfWriter{
	writersByComponent: map[string][]loki.LogsReceiver{},
}

// RegisterSelfReceivers is used to add or update global log receivers from a component.
func RegisterSelfReceivers(componentID string, f []loki.LogsReceiver) {
	selfLokiWriter.register(componentID, f)
}

// UnRegisterSelfReceivers is used to remove global log receivers when a component
// shuts down
func UnRegisterSelfReceivers(componentID string) {
	selfLokiWriter.unregister(componentID)
}
