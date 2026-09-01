package gather

import (
	"context"
	"net/url"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// sinkScheme is the zap sink scheme for log capture. A user routes collector
// logs to the support bundle by adding "supportbundle://" to
// service::telemetry::logs::output_paths.
const sinkScheme = "supportbundle"

// evictionNotice heads a snapshot whose oldest bytes were evicted.
const evictionNotice = "# [support bundle: older logs evicted; showing the most recent bytes]\n"

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

// LogSink is a zap sink that keeps the most recent collector logs in a ring
// buffer. The collector writes every log line to this sink when supportbundle://
// is in output_paths. The ring is created only when the operator sets a buffer
// size; until then Write is a lock-free no-op, so logging pays nothing.
type LogSink struct {
	ring atomic.Pointer[logRing]
}

func (s *LogSink) Write(p []byte) (int, error) {
	if r := s.ring.Load(); r != nil {
		return r.Write(p)
	}
	// Capture is disabled. Skip the lock entirely.
	return len(p), nil
}

// Sync does nothing. The sink writes to memory.
func (s *LogSink) Sync() error { return nil }

// Close does nothing. The sink lives for the whole process.
func (s *LogSink) Close() error { return nil }

// Enable turns on capture with a ring of the given size in bytes. A size of 0
// leaves capture disabled. The extension calls this at startup.
func (s *LogSink) Enable(size int) {
	if size > 0 {
		s.ring.Store(newLogRing(size))
	}
}

// snapshot returns the retained bytes, or nil when capture is disabled.
func (s *LogSink) snapshot() (data []byte, evicted bool) {
	if r := s.ring.Load(); r != nil {
		return r.snapshot()
	}
	return nil, false
}

// Logs writes the most recent collector logs to the bundle.
type Logs struct {
	Sink *LogSink
}

func (Logs) Name() string { return "logs" }

func (g Logs) Gather(_ context.Context, _ Options) ([]File, error) {
	data, evicted := g.Sink.snapshot()
	if len(data) == 0 {
		// Capture is off, or the user does not route logs to the sink.
		return nil, nil
	}
	if evicted {
		data = append([]byte(evictionNotice), data...)
	}
	return []File{{Path: "logs.txt", Content: data}}, nil
}
