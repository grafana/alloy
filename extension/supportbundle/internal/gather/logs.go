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

// evictionNotice heads the prior-history section when its oldest bytes were evicted.
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

// LogSink is a zap sink for the support bundle. The collector writes every log
// line to it when supportbundle:// is in output_paths. It captures logs two ways:
//
//   - A ring buffer holds the most recent bytes (the history PRIOR to a bundle).
//     It exists only when the operator sets a buffer size.
//   - A window buffer captures ALL log lines DURING a bundle, unbounded, so the
//     bundle keeps every line written while it is being built regardless of the
//     ring size. It is open only between beginWindow and endWindow.
//
// When neither is active, Write is a lock-free no-op.
type LogSink struct {
	ring atomic.Pointer[logRing]

	winActive atomic.Bool // lock-free fast path for the window buffer
	winMu     sync.Mutex
	winBuf    *bytes.Buffer // non-nil only while a bundle window is open
}

func (s *LogSink) Write(p []byte) (int, error) {
	if r := s.ring.Load(); r != nil {
		_, _ = r.Write(p)
	}
	if s.winActive.Load() {
		s.winMu.Lock()
		if s.winBuf != nil {
			s.winBuf.Write(p)
		}
		s.winMu.Unlock()
	}
	return len(p), nil
}

// Sync does nothing. The sink writes to memory.
func (s *LogSink) Sync() error { return nil }

// Close does nothing. The sink lives for the whole process.
func (s *LogSink) Close() error { return nil }

// Enable turns on the prior-history ring with the given size in bytes. A size of
// 0 leaves the ring off; the window buffer is unaffected. The extension calls
// this at startup.
func (s *LogSink) Enable(size int) {
	if size > 0 {
		s.ring.Store(newLogRing(size))
	}
}

// beginWindow snapshots the prior history from the ring and opens the window
// buffer so every subsequent line is captured in full. The window opens before
// the snapshot, so a line at the boundary is captured (possibly in both), never
// lost.
func (s *LogSink) beginWindow() (history []byte, evicted bool) {
	s.winMu.Lock()
	s.winBuf = &bytes.Buffer{}
	s.winMu.Unlock()
	s.winActive.Store(true)

	if r := s.ring.Load(); r != nil {
		history, evicted = r.snapshot()
	}
	return history, evicted
}

// endWindow closes the window buffer and returns the logs captured during it.
func (s *LogSink) endWindow() []byte {
	s.winActive.Store(false)
	s.winMu.Lock()
	defer s.winMu.Unlock()
	if s.winBuf == nil {
		return nil
	}
	out := append([]byte(nil), s.winBuf.Bytes()...)
	s.winBuf = nil
	return out
}

// Logs writes the collector logs to the bundle. It is an async gatherer: it
// snapshots the prior history at the start of the bundle and captures every line
// written during the bundle, then joins them into logs.txt.
type Logs struct {
	Sink *LogSink
}

func (Logs) Name() string { return "logs" }

func (g Logs) Start(_ context.Context, _ Options) (FinishFunc, error) {
	history, evicted := g.Sink.beginWindow()

	finish := func(_ context.Context) ([]File, error) {
		during := g.Sink.endWindow()
		if len(history) == 0 && len(during) == 0 {
			// Nothing captured: the user does not route logs to the sink.
			return nil, nil
		}

		var out []byte
		if evicted {
			out = append(out, evictionNotice...)
		}
		out = append(out, history...)
		out = append(out, during...)
		return []File{{Path: "logs.txt", Content: out}}, nil
	}

	return finish, nil
}
