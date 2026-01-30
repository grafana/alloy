package file

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
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
	l := logging.NewNop()
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
	tailer := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		func() bool { return true },
		sourceOptions{
			path:   logFile.Name(),
			labels: labels,
			fileWatch: FileWatch{
				MinPollFrequency: 25 * time.Millisecond,
				MaxPollFrequency: 25 * time.Millisecond,
			},
			onPositionsFileError: OnPositionsFileErrorRestartBeginning,
		},
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
	l := logging.NewNop()
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
	tailer := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		func() bool { return false },
		sourceOptions{
			path:   logFile.Name(),
			labels: labels,
			fileWatch: FileWatch{
				MinPollFrequency: 25 * time.Millisecond,
				MaxPollFrequency: 25 * time.Millisecond,
			},
			onPositionsFileError: OnPositionsFileErrorRestartBeginning,
		},
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
	l := logging.NewNop()
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
	tailer := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		func() bool { return true },
		sourceOptions{
			path:   logFile.Name(),
			labels: labels,
			fileWatch: FileWatch{
				MinPollFrequency: 25 * time.Millisecond,
				MaxPollFrequency: 25 * time.Millisecond,
			},
			onPositionsFileError: OnPositionsFileErrorRestartBeginning,
		},
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
	tailer := newTailer(
		newMetrics(nil),
		l,
		ch1,
		positionsFile,
		func() bool { return true },
		sourceOptions{
			path:   logFile.Name(),
			labels: labels,
			fileWatch: FileWatch{
				MinPollFrequency: 25 * time.Millisecond,
				MaxPollFrequency: 25 * time.Millisecond,
			},
			onPositionsFileError: OnPositionsFileErrorRestartEnd,
		},
	)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	// tailer needs some time to start
	time.Sleep(50 * time.Millisecond)

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

func TestTailer_Compressions(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	logger := log.NewNopLogger()
	positionsFile, err := positions.New(logger, positions.Config{
		SyncPeriod:    50 * time.Millisecond,
		PositionsFile: filepath.Join(t.TempDir(), "positions.yaml"),
	})
	require.NoError(t, err)
	defer positionsFile.Stop()

	filename := "testdata/onelinelog.tar.gz"
	labels := model.LabelSet{
		"filename": model.LabelValue(filename),
		"foo":      "bar",
	}

	tailer := newTailer(
		newMetrics(nil),
		logger,
		handler.Receiver(),
		positionsFile,
		func() bool { return true },
		sourceOptions{
			path:                 filename,
			labels:               labels,
			onPositionsFileError: OnPositionsFileErrorRestartBeginning,
			decompressionConfig:  DecompressionConfig{Enabled: true, Format: "gz"},
		},
	)

	// We expect tailer to exit when all compressed data have been consumed.
	tailer.Run(t.Context())

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		entries := handler.Received()
		require.Len(c, entries, 1)
		require.Contains(c, entries[0].Line, "onelinelog.log")
	}, 2*time.Second, 50*time.Millisecond)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pos, err := positionsFile.Get(filename, labels.String())
		assert.NoError(c, err)
		// FIXME: Previously we stored line posistion..
		assert.Equal(c, int64(10240), pos)
	}, time.Second, 50*time.Millisecond)

	handler.Clear()
	// Run the decompressor again
	tailer.Run(t.Context())

	entries := handler.Received()
	require.Len(t, entries, 0)
}

func TestTailer_GigantiqueGunzipFile(t *testing.T) {
	file := "testdata/long-access.gz"
	handler := loki.NewCollectingHandler()
	defer handler.Stop()

	tailer := newTailer(
		newMetrics(prometheus.NewRegistry()),
		log.NewNopLogger(),
		handler.Receiver(),
		positions.NewNop(),
		func() bool { return false },
		sourceOptions{
			path:                file,
			decompressionConfig: DecompressionConfig{Enabled: true, Format: "gz"},
		},
	)

	// We expect tailer to exit when all compressed data have been consumed.
	tailer.Run(t.Context())

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Equal(c, 100000, len(handler.Received()))
	}, 2*time.Second, 50*time.Millisecond)
}

// TestTailer_CompressedOnelineFile test the supported formats for log lines that only contain 1 line.
// Based on our experience, this is the scenario with the most edge cases.
func TestTailer_CompressedOnelineFile(t *testing.T) {
	fileContent, err := os.ReadFile("testdata/onelinelog.log")
	require.NoError(t, err)
	t.Run("gunzip file", func(t *testing.T) {
		file := "testdata/onelinelog.log.gz"
		handler := loki.NewCollectingHandler()
		defer handler.Stop()

		tailer := newTailer(
			newMetrics(prometheus.NewRegistry()),
			log.NewNopLogger(),
			handler.Receiver(),
			positions.NewNop(),
			func() bool { return false },
			sourceOptions{
				path:                file,
				decompressionConfig: DecompressionConfig{Enabled: true, Format: "gz"},
			},
		)
		require.NoError(t, err)

		// We expect tailer to exit when all compressed data have been consumed.
		tailer.Run(t.Context())

		require.Eventually(t, func() bool {
			return len(handler.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		entries := handler.Received()
		require.Equal(t, string(fileContent), entries[0].Line)
	})

	t.Run("bzip2 file", func(t *testing.T) {
		file := "testdata/onelinelog.log.bz2"
		handler := loki.NewCollectingHandler()
		defer handler.Stop()

		tailer := newTailer(
			newMetrics(prometheus.NewRegistry()),
			log.NewNopLogger(),
			handler.Receiver(),
			positions.NewNop(),
			func() bool { return false },
			sourceOptions{
				path:                file,
				decompressionConfig: DecompressionConfig{Enabled: true, Format: "bz2"},
			},
		)

		// We expect tailer to exit when all compressed data have been consumed.
		tailer.Run(t.Context())

		require.Eventually(t, func() bool {
			return len(handler.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		entries := handler.Received()
		require.Equal(t, string(fileContent), entries[0].Line)
	})

	t.Run("tar.gz file", func(t *testing.T) {
		file := "testdata/onelinelog.tar.gz"
		handler := loki.NewCollectingHandler()
		defer handler.Stop()

		tailer := newTailer(
			newMetrics(prometheus.NewRegistry()),
			log.NewNopLogger(),
			handler.Receiver(),
			positions.NewNop(),
			func() bool { return false },
			sourceOptions{
				path:                file,
				decompressionConfig: DecompressionConfig{Enabled: true, Format: "gz"},
			},
		)
		require.NoError(t, err)

		// We expect tailer to exit when all compressed data have been consumed.
		tailer.Run(t.Context())

		require.Eventually(t, func() bool {
			return len(handler.Received()) == 1
		}, 2*time.Second, 50*time.Millisecond)

		entries := handler.Received()
		require.Contains(t, entries[0].Line, "onelinelog.log") // contains .tar.gz headers
		require.Contains(
			t,
			entries[0].Line,
			`5.202.214.160 - - [26/Jan/2019:19:45:25 +0330] "GET / HTTP/1.1" 200 30975 "https://www.zanbil.ir/" "Mozilla/5.0 (Windows NT 6.2; WOW64; rv:21.0) Gecko/20100101 Firefox/21.0" "-"`,
		)
	})
}
