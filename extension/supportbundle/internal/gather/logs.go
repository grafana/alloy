package gather

import (
	"bytes"
	"context"
	"net/url"
	"sync"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// sinkScheme is the zap sink scheme for log capture. A user routes collector
// logs to the support bundle by adding "supportbundle://" to
// service::telemetry::logs::output_paths.
const sinkScheme = "supportbundle"

// truncationNotice marks a captured log that reached the buffer limit.
const truncationNotice = "\n[support bundle: log capture truncated at buffer limit]\n"

// LogCapture holds the process-wide log capture. zap registers sinks per
// process, and the collector builds its logger once, so this state must be
// process-global. LogCapture keeps that global surface to a single value.
var LogCapture = newLogCapture()

// LogCaptureState is the process-wide sink plus any registration error.
type LogCaptureState struct {
	Sink *LogSink
	Err  error
}

// newLogCapture creates the sink and registers it with zap. It runs once, when
// the package loads, so the sink exists before the collector builds its logger.
func newLogCapture() *LogCaptureState {
	lc := &LogCaptureState{Sink: &LogSink{}}
	lc.Err = zap.RegisterSink(sinkScheme, func(_ *url.URL) (zap.Sink, error) {
		return lc.Sink, nil
	})
	return lc
}

// LogSink is a zap sink that captures logs while a support bundle runs.
// The collector writes every log line to this sink. The sink keeps the lines
// only while a capture window is open. It discards them otherwise, so the cost
// is low when no bundle runs.
type LogSink struct {
	// capturing is a lock-free fast path. It is true only while a bundle runs.
	// The collector logs every line to this sink when supportbundle:// is in
	// output_paths, so the idle path must not take the lock.
	capturing atomic.Bool

	mu        sync.Mutex
	buf       *bytes.Buffer // non-nil only while capturing
	limit     int           // maximum bytes to keep; 0 disables the limit
	truncated bool
}

func (s *LogSink) Write(p []byte) (int, error) {
	// Fast path: no bundle is capturing. Skip the lock entirely.
	if !s.capturing.Load() {
		return len(p), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buf == nil {
		return len(p), nil
	}

	if s.limit > 0 {
		remaining := s.limit - s.buf.Len()
		if remaining <= 0 {
			s.truncated = true
			return len(p), nil
		}
		if len(p) > remaining {
			s.buf.Write(p[:remaining])
			s.truncated = true
			return len(p), nil
		}
	}

	s.buf.Write(p)
	return len(p), nil
}

// Sync does nothing. The sink writes to memory.
func (s *LogSink) Sync() error { return nil }

// Close does nothing. The sink lives for the whole process.
func (s *LogSink) Close() error { return nil }

// start opens a capture window with a fresh buffer and the given byte limit.
func (s *LogSink) start(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = &bytes.Buffer{}
	s.limit = limit
	s.truncated = false
	// Set the fast-path flag last, so writers only take the lock once the
	// buffer is ready.
	s.capturing.Store(true)
}

// stop closes the capture window. It returns a copy of the captured logs and
// reports whether the buffer reached its limit.
func (s *LogSink) stop() (data []byte, truncated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Clear the fast-path flag first, so new writes skip the lock immediately.
	s.capturing.Store(false)
	if s.buf == nil {
		return nil, false
	}
	out := append([]byte(nil), s.buf.Bytes()...)
	truncated = s.truncated
	s.buf = nil
	return out, truncated
}

// Logs captures collector logs over the collection window.
type Logs struct {
	Sink  *LogSink
	Limit int
}

func (Logs) Name() string { return "logs" }

func (g Logs) Start(_ context.Context, _ Options) (FinishFunc, error) {
	g.Sink.start(g.Limit)

	finish := func(_ context.Context) ([]File, error) {
		data, truncated := g.Sink.stop()
		if len(data) == 0 {
			// No logs were captured. The user may not route logs to the sink.
			return nil, nil
		}
		if truncated {
			data = append(data, truncationNotice...)
		}
		return []File{{Path: "logs.txt", Content: data}}, nil
	}

	return finish, nil
}
