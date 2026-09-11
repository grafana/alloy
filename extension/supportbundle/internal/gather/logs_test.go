package gather

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// runLogs runs the async Logs gatherer. Anything written to the sink before the
// call lands in the prior history (if the ring is enabled); anything the writeFn
// writes lands in the during-window capture.
func runLogs(t *testing.T, sink *LogSink, writeDuring func()) ([]File, error) {
	t.Helper()
	finish, err := Logs{Sink: sink}.Start(context.Background(), Options{})
	require.NoError(t, err)
	require.NotNil(t, finish)
	if writeDuring != nil {
		writeDuring()
	}
	return finish(context.Background())
}

func TestLogsNothingCaptured(t *testing.T) {
	// Not routed / nothing written: no logs.txt.
	files, err := runLogs(t, &LogSink{}, nil)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestLogsPriorHistoryFromRing(t *testing.T) {
	s := &LogSink{}
	s.Enable(1024)
	_, _ = s.Write([]byte("before-bundle\n")) // prior -> ring

	files, err := runLogs(t, s, nil)
	require.NoError(t, err)

	got := string(gatherToMap(t, files)["logs.txt"])
	require.Contains(t, got, "before-bundle")
}

func TestLogsDuringWindowWithoutRing(t *testing.T) {
	// Even with the ring disabled, logs written during the bundle are captured.
	s := &LogSink{} // ring off
	files, err := runLogs(t, s, func() {
		_, _ = s.Write([]byte("during-bundle\n"))
	})
	require.NoError(t, err)

	got := string(gatherToMap(t, files)["logs.txt"])
	require.Contains(t, got, "during-bundle")
}

func TestLogsHistoryThenDuring(t *testing.T) {
	s := &LogSink{}
	s.Enable(1024)
	_, _ = s.Write([]byte("prior-line\n"))

	files, err := runLogs(t, s, func() {
		_, _ = s.Write([]byte("window-line\n"))
	})
	require.NoError(t, err)

	got := string(gatherToMap(t, files)["logs.txt"])
	require.Contains(t, got, "prior-line")
	require.Contains(t, got, "window-line")
	// Prior history comes before the during-window logs.
	require.Less(t, strings.Index(got, "prior-line"), strings.Index(got, "window-line"))
}

func TestLogsDuringWindowIsUnbounded(t *testing.T) {
	// A tiny ring caps the prior history, but the during-window capture keeps
	// every line regardless of the ring size.
	s := &LogSink{}
	s.Enable(16)

	files, err := runLogs(t, s, func() {
		for i := 0; i < 100; i++ {
			_, _ = s.Write([]byte("0123456789\n")) // 11 bytes each, 1100 total
		}
	})
	require.NoError(t, err)

	got := gatherToMap(t, files)["logs.txt"]
	require.Greater(t, len(got), 16*4) // far more than the 16-byte ring
}

func TestLogsPriorEvictionNotice(t *testing.T) {
	s := &LogSink{}
	s.Enable(16) // tiny ring, forces wrap on the prior history
	for i := 0; i < 100; i++ {
		_, _ = s.Write([]byte("0123456789\n"))
	}

	files, err := runLogs(t, s, nil)
	require.NoError(t, err)

	require.Contains(t, string(gatherToMap(t, files)["logs.txt"]), evictionNotice)
}

func TestLogsViaZapLogger(t *testing.T) {
	require.NoError(t, LogCapture.Err)
	LogCapture.Sink.Enable(1 << 16)

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{sinkScheme + "://"}
	logger, err := cfg.Build()
	require.NoError(t, err)

	logger.Info("before-window") // prior -> ring
	_ = logger.Sync()

	files, err := runLogs(t, LogCapture.Sink, func() {
		logger.Info("inside-window") // during -> window
		_ = logger.Sync()
	})
	require.NoError(t, err)

	got := string(gatherToMap(t, files)["logs.txt"])
	require.Contains(t, got, "before-window")
	require.Contains(t, got, "inside-window")
}
