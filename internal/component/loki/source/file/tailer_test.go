package file

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/tail/watch"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func createTempFileWithContent(t *testing.T, content []byte) string {
	t.Helper()
	tmpfile, err := os.CreateTemp(t.TempDir(), "testfile")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err = tmpfile.Write(content)
	if err != nil {
		tmpfile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	tmpfile.Close()
	return tmpfile.Name()
}

func TestGetLastLinePosition(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected int64
	}{
		{
			name:     "File ending with newline",
			content:  []byte("Hello, World!\n"),
			expected: 14, // Position after last '\n'
		},
		{
			name:     "Newline in the middle",
			content:  []byte("Hello\nWorld"),
			expected: 6, // Position after the '\n' in "Hello\n"
		},
		{
			name:     "File not ending with newline",
			content:  []byte("Hello, World!"),
			expected: 0,
		},
		{
			name:     "File bigger than chunkSize without newline",
			content:  bytes.Repeat([]byte("A"), 1025),
			expected: 0,
		},
		{
			name:     "File bigger than chunkSize with newline in between",
			content:  append([]byte("Hello\n"), bytes.Repeat([]byte("A"), 1025)...),
			expected: 6, // Position after the "Hello\n"
		},
		{
			name:     "Empty file",
			content:  []byte(""),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := createTempFileWithContent(t, tt.content)
			defer os.Remove(filename)

			got, err := getLastLinePosition(filename)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.expected {
				t.Errorf("for content %q, expected position %d but got %d", tt.content, tt.expected, got)
			}
		})
	}
}

func TestTailer(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	l := util.TestLogger(t)
	ch1 := loki.NewLogsReceiver()
	tempDir := t.TempDir()
	logFile, err := os.CreateTemp(tempDir, "example")
	require.NoError(t, err)
	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	labels := model.LabelSet{
		"filename": model.LabelValue(logFile.Name()),
		"foo":      "bar",
	}
	tailer, err := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		logFile.Name(),
		labels,
		"",
		watch.PollingFileWatcherOptions{
			MinPollFrequency: 25 * time.Millisecond,
			MaxPollFrequency: 25 * time.Millisecond,
		},
		false,
		false,
		OnPositionsFileErrorRestartBeginning,
		func() bool { return true },
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	_, err = logFile.Write([]byte("writing some text\n"))
	require.NoError(t, err)
	select {
	case logEntry := <-ch1.Chan():
		require.Equal(t, "writing some text", logEntry.Line)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(logFile.Name(), labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(18), pos)
	}, time.Second, 50*time.Millisecond)

	cancel()
	<-done

	// Run the tailer again
	ctx, cancel = context.WithCancel(t.Context())
	done = make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	select {
	case <-ch1.Chan():
		t.Fatal("no message should be sent because of the position file")
	case <-time.After(1 * time.Second):
	}

	// Write logs again
	_, err = logFile.Write([]byte("writing some new text\n"))
	require.NoError(t, err)
	select {
	case logEntry := <-ch1.Chan():
		require.Equal(t, "writing some new text", logEntry.Line)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	cancel()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(logFile.Name(), labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(40), pos)
	}, time.Second, 50*time.Millisecond)

	positionsFile.Stop()
	require.NoError(t, logFile.Close())
}

func TestTailerPositionFileEntryDeleted(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	l := util.TestLogger(t)
	ch1 := loki.NewLogsReceiver()
	tempDir := t.TempDir()
	logFile, err := os.CreateTemp(tempDir, "example")
	require.NoError(t, err)
	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	labels := model.LabelSet{
		"filename": model.LabelValue(logFile.Name()),
		"foo":      "bar",
	}
	tailer, err := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		logFile.Name(),
		labels,
		"",
		watch.PollingFileWatcherOptions{
			MinPollFrequency: 25 * time.Millisecond,
			MaxPollFrequency: 25 * time.Millisecond,
		},
		false,
		false,
		OnPositionsFileErrorRestartBeginning,
		func() bool { return false },
	)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	go tailer.Run(ctx)

	_, err = logFile.Write([]byte("writing some text\n"))
	require.NoError(t, err)
	select {
	case logEntry := <-ch1.Chan():
		require.Equal(t, "writing some text", logEntry.Line)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(logFile.Name(), labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(18), pos)
	}, time.Second, 50*time.Millisecond)

	cancel()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(logFile.Name(), labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(0), pos)
	}, time.Second, 50*time.Millisecond)

	positionsFile.Stop()
	require.NoError(t, logFile.Close())
}

func TestTailerDeleteFileInstant(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	l := util.TestLogger(t)
	ch1 := loki.NewLogsReceiver()
	tempDir := t.TempDir()
	logFile, err := os.CreateTemp(tempDir, "example")
	require.NoError(t, err)
	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	labels := model.LabelSet{
		"filename": model.LabelValue(logFile.Name()),
		"foo":      "bar",
	}
	tailer, err := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		logFile.Name(),
		labels,
		"",
		watch.PollingFileWatcherOptions{
			MinPollFrequency: 25 * time.Millisecond,
			MaxPollFrequency: 25 * time.Millisecond,
		},
		false,
		false,
		OnPositionsFileErrorRestartBeginning,
		func() bool { return true },
	)
	require.NoError(t, err)

	// Close the file before running the tailer
	require.NoError(t, logFile.Close())
	require.NoError(t, os.Remove(logFile.Name()))

	done := make(chan struct{})
	go func() {
		tailer.Run(t.Context())
		positionsFile.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tailer deadlocked")
	}
}

func TestTailerCorruptedPositions(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	l := util.TestLogger(t)
	ch1 := loki.NewLogsReceiver()
	tempDir := t.TempDir()
	logFile, err := os.CreateTemp(tempDir, "example")
	require.NoError(t, err)
	_, err = logFile.Write([]byte("initial content\n"))
	require.NoError(t, err)

	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	labels := model.LabelSet{
		"filename": model.LabelValue(logFile.Name()),
		"foo":      "bar",
	}
	positionsFile.PutString(logFile.Name(), labels.String(), "\\0\\0\\0\\0123") // Corrupted position entry
	tailer, err := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		logFile.Name(),
		labels,
		"",
		watch.PollingFileWatcherOptions{
			MinPollFrequency: 25 * time.Millisecond,
			MaxPollFrequency: 25 * time.Millisecond,
		},
		false,
		false,
		OnPositionsFileErrorRestartEnd,
		func() bool { return true },
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, tailer.IsRunning())
		assert.Equal(c, "16", positionsFile.GetString(logFile.Name(), labels.String()))
	}, time.Second, 50*time.Millisecond)

	_, err = logFile.Write([]byte("writing some text\n"))
	require.NoError(t, err)
	select {
	case logEntry := <-ch1.Chan():
		require.Equal(t, "writing some text", logEntry.Line)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(logFile.Name(), labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(34), pos)
	}, time.Second, 50*time.Millisecond)

	cancel()
	<-done

	// Run the tailer again
	ctx, cancel = context.WithCancel(t.Context())
	done = make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	select {
	case <-ch1.Chan():
		t.Fatal("no message should be sent because of the position file")
	case <-time.After(1 * time.Second):
	}

	// Write logs again
	_, err = logFile.Write([]byte("writing some new text\n"))
	require.NoError(t, err)
	select {
	case logEntry := <-ch1.Chan():
		require.Equal(t, "writing some new text", logEntry.Line)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}

	cancel()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(logFile.Name(), labels.String())
		assert.NoError(c, err)
		assert.Equal(c, int64(56), pos)
	}, time.Second, 50*time.Millisecond)

	positionsFile.Stop()
	require.NoError(t, logFile.Close())
}
