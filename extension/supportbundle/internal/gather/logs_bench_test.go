package gather

import (
	"io"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// benchLine is a representative encoded log line (~180 bytes).
var benchLine = []byte(`{"level":"info","ts":"2026-09-01T00:00:00.000Z","logger":"otelcol.receiver.otlp","msg":"processing batch","component":"otlp","pipeline":"traces","count":512,"bytes":40960}` + "\n")

const benchRingSize = 1 << 20 // 1 MiB

// Disabled sink: Write is a lock-free no-op (the common case).
func BenchmarkLogSinkDisabled(b *testing.B) {
	s := &LogSink{}
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.Write(benchLine)
	}
}

func BenchmarkLogSinkDisabledParallel(b *testing.B) {
	s := &LogSink{}
	b.SetBytes(int64(len(benchLine)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Write(benchLine)
		}
	})
}

// Enabled sink: every write goes into the ring under its mutex.
func BenchmarkLogSinkEnabled(b *testing.B) {
	s := &LogSink{}
	s.Enable(benchRingSize)
	b.SetBytes(int64(len(benchLine)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.Write(benchLine)
	}
}

func BenchmarkLogSinkEnabledParallel(b *testing.B) {
	s := &LogSink{}
	s.Enable(benchRingSize)
	b.SetBytes(int64(len(benchLine)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Write(benchLine)
		}
	})
}

func benchLogger(ws zapcore.WriteSyncer) *zap.Logger {
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	return zap.New(zapcore.NewCore(enc, ws, zapcore.InfoLevel))
}

func logOnce(logger *zap.Logger) {
	logger.Info("processing batch",
		zap.String("component", "otlp"),
		zap.String("pipeline", "traces"),
		zap.Int("count", 512),
	)
}

// Baseline: full zap logger to a lock-free discard syncer.
func BenchmarkLoggerDiscard(b *testing.B) {
	logger := benchLogger(zapcore.AddSync(io.Discard))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logOnce(logger)
		}
	})
}

// The delta from Discard is the cost of routing logs to a disabled sink.
func BenchmarkLoggerSinkDisabled(b *testing.B) {
	logger := benchLogger(&LogSink{})
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logOnce(logger)
		}
	})
}

// The delta from Discard is the always-on cost of the enabled ring.
func BenchmarkLoggerSinkEnabled(b *testing.B) {
	s := &LogSink{}
	s.Enable(benchRingSize)
	logger := benchLogger(s)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logOnce(logger)
		}
	})
}
