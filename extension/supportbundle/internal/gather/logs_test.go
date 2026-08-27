package gather

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newSinkLogger builds a zap logger that writes to the support bundle sink.
// The sink is registered when the package loads.
func newSinkLogger(t *testing.T) *zap.Logger {
	t.Helper()
	require.NoError(t, LogCapture.Err)

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{sinkScheme + "://"}
	logger, err := cfg.Build()
	require.NoError(t, err)
	return logger
}

func TestLogGathererCapturesWindow(t *testing.T) {
	logger := newSinkLogger(t)

	// A line logged before the window opens is not captured.
	logger.Info("before-window")
	_ = logger.Sync()

	g := Logs{Sink: LogCapture.Sink}
	finish, err := g.Start(context.Background(), Options{})
	require.NoError(t, err)

	logger.Info("inside-window")
	_ = logger.Sync()

	files, err := finish(context.Background())
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "logs.txt")
	require.Contains(t, string(m["logs.txt"]), "inside-window")
	require.NotContains(t, string(m["logs.txt"]), "before-window")

	// A line logged after the window closes is not captured.
	logger.Info("after-window")
	_ = logger.Sync()
	data, _ := LogCapture.Sink.stop()
	require.NotContains(t, string(data), "after-window")
}

func TestLogGathererNoLogsNoFile(t *testing.T) {
	// The window opens and closes with no logs routed to the sink.
	g := Logs{Sink: &LogSink{}}
	finish, err := g.Start(context.Background(), Options{})
	require.NoError(t, err)

	files, err := finish(context.Background())
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestLogSinkTruncatesAtLimit(t *testing.T) {
	s := &LogSink{}
	s.start(10)

	// Zap sees a full write, but the sink keeps only the first 10 bytes.
	n, err := s.Write([]byte("0123456789ABCDEF"))
	require.NoError(t, err)
	require.Equal(t, 16, n)

	data, truncated := s.stop()
	require.True(t, truncated)
	require.Equal(t, "0123456789", string(data))
}

func TestLogGathererAddsTruncationNotice(t *testing.T) {
	s := &LogSink{}
	g := Logs{Sink: s, Limit: 5}

	finish, err := g.Start(context.Background(), Options{})
	require.NoError(t, err)

	_, err = s.Write([]byte("hello world"))
	require.NoError(t, err)

	files, err := finish(context.Background())
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "logs.txt")
	require.Contains(t, string(m["logs.txt"]), "hello")
	require.Contains(t, string(m["logs.txt"]), "truncated at buffer limit")
}
