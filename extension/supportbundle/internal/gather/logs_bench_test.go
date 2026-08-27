package gather

import (
	"io"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// benchLine is a representative encoded log line (~180 bytes).
var benchLine = []byte(`{"level":"info","ts":"2026-08-27T00:00:00.000Z","logger":"otelcol.receiver.otlp","msg":"processing batch","component":"otlp","pipeline":"traces","count":512,"bytes":40960}` + "\n")

// BenchmarkLogSinkWriteIdle measures the per-write cost when no bundle is
// running. This is the common case: the sink is in the output path but only
// checks the atomic flag and returns.
func BenchmarkLogSinkWriteIdle(b *testing.B) {
	s := &LogSink{} // not capturing
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Write(benchLine); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLogSinkWriteIdleParallel measures idle-write cost under concurrent
// logging. The lock-free fast path must not contend.
func BenchmarkLogSinkWriteIdleParallel(b *testing.B) {
	s := &LogSink{}
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := s.Write(benchLine); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLogSinkWriteCapturing measures the per-write cost while a bundle is
// capturing. The buffer is reset periodically to bound memory.
func BenchmarkLogSinkWriteCapturing(b *testing.B) {
	s := &LogSink{}
	s.start(0) // unlimited
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Write(benchLine); err != nil {
			b.Fatal(err)
		}
		if s.buf.Len() > 8<<20 {
			s.start(0)
		}
	}
}

// benchLogger builds a JSON zap logger writing to the given syncer.
func benchLogger(ws zapcore.WriteSyncer) *zap.Logger {
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	return zap.New(zapcore.NewCore(enc, ws, zapcore.InfoLevel))
}

func logOnce(logger *zap.Logger) {
	logger.Info("processing batch",
		zap.String("component", "otlp"),
		zap.String("pipeline", "traces"),
		zap.Int("count", 512),
		zap.Int("bytes", 40960),
	)
}

// BenchmarkLoggerDiscard is the baseline: a zap logger to a lock-free discard
// syncer, logging concurrently.
func BenchmarkLoggerDiscard(b *testing.B) {
	logger := benchLogger(zapcore.AddSync(io.Discard))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logOnce(logger)
		}
	})
}

// BenchmarkLoggerSupportBundleSinkIdle logs concurrently to the support bundle
// sink while idle. The delta from BenchmarkLoggerDiscard is the cost of adding
// "supportbundle://" to output_paths.
func BenchmarkLoggerSupportBundleSinkIdle(b *testing.B) {
	logger := benchLogger(&LogSink{})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logOnce(logger)
		}
	})
}
