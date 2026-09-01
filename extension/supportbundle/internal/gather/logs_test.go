package gather

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLogSinkDisabledIsNoOp(t *testing.T) {
	s := &LogSink{} // no ring configured
	n, err := s.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)

	data, evicted := s.snapshot()
	require.Nil(t, data)
	require.False(t, evicted)

	files, err := Logs{Sink: s}.Gather(context.Background(), Options{})
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestLogSinkCaptures(t *testing.T) {
	s := &LogSink{}
	s.Enable(1024)
	_, _ = s.Write([]byte("line one\n"))
	_, _ = s.Write([]byte("line two\n"))

	files, err := Logs{Sink: s}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "logs.txt")
	got := string(m["logs.txt"])
	require.Contains(t, got, "line one")
	require.Contains(t, got, "line two")
	require.NotContains(t, got, evictionNotice) // fits, nothing evicted
}

func TestLogSinkEvictsOldest(t *testing.T) {
	s := &LogSink{}
	s.Enable(16) // tiny ring, forces wrap
	for i := 0; i < 100; i++ {
		_, _ = s.Write([]byte("0123456789\n")) // 11 bytes each
	}

	files, err := Logs{Sink: s}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	got := string(gatherToMap(t, files)["logs.txt"])
	require.Contains(t, got, evictionNotice)                 // wrapped -> notice
	require.LessOrEqual(t, len(got)-len(evictionNotice), 16) // retained <= ring size
}

func TestLogSinkViaZapLogger(t *testing.T) {
	require.NoError(t, LogCapture.Err)
	LogCapture.Sink.Enable(1 << 16)

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{sinkScheme + "://"}
	logger, err := cfg.Build()
	require.NoError(t, err)

	logger.Info("via-zap-marker")
	_ = logger.Sync()

	files, err := Logs{Sink: LogCapture.Sink}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	require.Contains(t, string(gatherToMap(t, files)["logs.txt"]), "via-zap-marker")
}
