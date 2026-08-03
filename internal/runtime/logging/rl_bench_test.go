package logging

import (
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// newRLBenchLogger builds a real *Logger that writes to io.Discard, using
// the given RateLimitingOptions. It sets up a fresh metrics registry, so the
// drop path exercises the alloy_logging_suppressed_lines_total counter, the
// same as in production.
func newRLBenchLogger(b *testing.B, rl RateLimitingOptions) *Logger {
	b.Helper()
	l, err := New(io.Discard, Options{
		Level:        LevelInfo,
		Format:       FormatLogfmt,
		RateLimiting: &rl,
	})
	if err != nil {
		b.Fatalf("failed to create logger: %v", err)
	}
	l.InitRateLimitMetrics(prometheus.NewRegistry())
	return l
}

// BenchmarkRL_Disabled measures the baseline passthrough path with rate
// limiting disabled entirely.
func BenchmarkRL_Disabled(b *testing.B) {
	l := newRLBenchLogger(b, RateLimitingOptions{Enabled: false})
	log := l.Slog()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("bench message")
	}
}

// BenchmarkRL_AdmitHot measures the steady-state admit path. The threshold
// is effectively unbounded, so every call is admitted: build the matcher
// key, increase the counter, and write to the terminal.
func BenchmarkRL_AdmitHot(b *testing.B) {
	l := newRLBenchLogger(b, RateLimitingOptions{
		Enabled:       true,
		Tick:          time.Hour,
		Threshold:     1 << 62,
		Rate:          0,
		MaxSignatures: 1000,
	})
	log := l.Slog()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("bench message")
	}
}

// BenchmarkRL_DropHot measures the drop path. After the first call, every
// identical call is dropped: matcher, counter, and OnDropped metric, but no
// terminal write.
func BenchmarkRL_DropHot(b *testing.B) {
	l := newRLBenchLogger(b, RateLimitingOptions{
		Enabled:       true,
		Tick:          time.Hour,
		Threshold:     1,
		Rate:          0,
		MaxSignatures: 1000,
	})
	log := l.Slog()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("bench message")
	}
}

// BenchmarkRL_Churn logs a different message each iteration so every call
// is a new signature, exercising the LRU insert path.
func BenchmarkRL_Churn(b *testing.B) {
	l := newRLBenchLogger(b, RateLimitingOptions{
		Enabled:       true,
		Tick:          time.Hour,
		Threshold:     1 << 62,
		Rate:          0,
		MaxSignatures: 1000,
	})
	log := l.Slog()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("msg " + strconv.Itoa(i&0xffff))
	}
}

// BenchmarkRL_Parallel measures lock/contention on the shared buffer when N
// goroutines log the same admit-hot line concurrently.
func BenchmarkRL_Parallel(b *testing.B) {
	l := newRLBenchLogger(b, RateLimitingOptions{
		Enabled:       true,
		Tick:          time.Hour,
		Threshold:     1 << 62,
		Rate:          0,
		MaxSignatures: 1000,
	})
	log := l.Slog()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			log.Info("bench message")
		}
	})
}
