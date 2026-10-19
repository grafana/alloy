package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestTailer(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
		l := logging.NewSlogNop()
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
			newBatcher,
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

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			tailer.Run(ctx)
			close(done)
		}()

		_, err = logFile.Write([]byte("writing some text\n"))
		require.NoError(t, err)
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			entries := received()
			require.Len(c, entries, 1)
			require.Equal(c, "writing some text", entries[0].Line)
		}, time.Second, 10*time.Millisecond)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(logFile.Name(), labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(18), pos)
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

		// No entry should be sent because of the position file.
		require.Never(t, func() bool {
			return len(received()) > 1
		}, time.Second, 50*time.Millisecond)

		// Write logs again
		_, err = logFile.Write([]byte("writing some new text\n"))
		require.NoError(t, err)
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			entries := received()
			require.Len(c, entries, 2)
			require.Equal(c, "writing some new text", entries[1].Line)
		}, time.Second, 10*time.Millisecond)

		cancel()
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(logFile.Name(), labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(40), pos)
		}, time.Second, 50*time.Millisecond)
		<-done

		positionsFile.Stop()
		require.NoError(t, logFile.Close())
	})
}

func TestTailerPositionFileEntryDeleted(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
		l := logging.NewSlogNop()
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
			newBatcher,
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
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			tailer.Run(ctx)
			close(done)
		}()

		_, err = logFile.Write([]byte("writing some text\n"))
		require.NoError(t, err)
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			entries := received()
			require.Len(c, entries, 1)
			require.Equal(c, "writing some text", entries[0].Line)
		}, time.Second, 10*time.Millisecond)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(logFile.Name(), labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(18), pos)
		}, time.Second, 50*time.Millisecond)

		cancel()
		<-done

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(logFile.Name(), labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(0), pos)
		}, time.Second, 50*time.Millisecond)

		positionsFile.Stop()
		require.NoError(t, logFile.Close())
	})
}

func TestTailerDeleteFileInstant(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, _ func() []loki.Entry) {
		l := logging.NewSlogNop()
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
			newBatcher,
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
	})
}

func TestTailerCancelWhileSendBlocked(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	t.Run("pipeline v1", func(t *testing.T) {
		recv := loki.NewLogsReceiver()
		testTailerCancelWhileSendBlocked(t, channelBatcher(recv), func() string {
			entry := <-recv.Chan()
			return entry.Line
		})
	})

	t.Run("pipeline v2", func(t *testing.T) {
		consumer := &blockingConsumer{CollectingConsumer: loki.NewCollectingConsumer()}
		// Batch size 1 so the second line is sent in its own Consume call.
		testTailerCancelWhileSendBlocked(t, consumerBatcher(consumer, 1), func() string {
			var entries []loki.Entry
			require.Eventually(t, func() bool {
				entries = consumer.Entries()
				return len(entries) > 0
			}, time.Second, 10*time.Millisecond)
			return entries[0].Line
		})
	})
}

func testTailerCancelWhileSendBlocked(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, firstLine func() string) {
	l := logging.NewSlogNop()
	tempDir := t.TempDir()

	file, err := os.CreateTemp(tempDir, "example")
	require.NoError(t, err)
	_, err = file.Write([]byte("1\n"))
	require.NoError(t, err)
	_, err = file.Write([]byte("2\n"))
	require.NoError(t, err)
	file.Close()

	positionsFile, err := positions.New(l, positions.Config{
		SyncPeriod:        50 * time.Millisecond,
		PositionsFile:     filepath.Join(tempDir, "positions.yaml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	require.NoError(t, err)
	defer positionsFile.Stop()

	var (
		path   = file.Name()
		labels = model.LabelSet{
			"filename": model.LabelValue(file.Name()),
			"foo":      "bar",
		}
	)

	tailer := newTailer(
		newMetrics(nil),
		l,
		newBatcher,
		positionsFile,
		func() bool { return true },
		sourceOptions{
			path:   path,
			labels: labels,
			fileWatch: FileWatch{
				MinPollFrequency: 25 * time.Millisecond,
				MaxPollFrequency: 25 * time.Millisecond,
			},
			onPositionsFileError: OnPositionsFileErrorRestartBeginning,
		},
	)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		tailer.Run(ctx)
		close(done)
	}()

	require.Equal(t, "1", firstLine())

	// Stop tailer while the send of the second line is blocked.
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tailer deadlocked while send was blocked")
	}

	pos, err := positionsFile.Get(path, labels.String())
	require.NoError(t, err)
	require.Equal(t, int64(2), pos)
}

type blockingConsumer struct {
	*loki.CollectingConsumer
}

func (c *blockingConsumer) Consume(ctx context.Context, batch loki.Batch) error {
	if len(c.Entries()) > 0 {
		<-ctx.Done()
		return ctx.Err()
	}
	return c.CollectingConsumer.Consume(ctx, batch)
}

func TestTailerCorruptedPositions(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
		l := logging.NewSlogNop()
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
		defer positionsFile.Stop()

		labels := model.LabelSet{
			"filename": model.LabelValue(logFile.Name()),
			"foo":      "bar",
		}
		positionsFile.PutString(logFile.Name(), labels.String(), "\\0\\0\\0\\0123") // Corrupted position entry
		tailer := newTailer(
			newMetrics(nil),
			l,
			newBatcher,
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
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			entries := received()
			require.Len(c, entries, 1)
			require.Equal(c, "writing some text", entries[0].Line)
		}, time.Second, 10*time.Millisecond)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(logFile.Name(), labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(34), pos)
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

		// No entry should be sent because of the position file.
		require.Never(t, func() bool {
			return len(received()) > 1
		}, time.Second, 50*time.Millisecond)

		// Write logs again
		_, err = logFile.Write([]byte("writing some new text\n"))
		require.NoError(t, err)
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			entries := received()
			require.Len(c, entries, 2)
			require.Equal(c, "writing some new text", entries[1].Line)
		}, time.Second, 10*time.Millisecond)

		cancel()
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(logFile.Name(), labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(56), pos)
		}, time.Second, 50*time.Millisecond)

		<-done
		require.NoError(t, logFile.Close())
	})
}

func TestTailer_Compressions(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
		logger := logging.NewSlogNop()
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
			newBatcher,
			positionsFile,
			// We return false here to verify that position is kept and we don't re-ingest the file.
			func() bool { return false },
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
			entries := received()
			require.Len(c, entries, 1)
			require.Contains(c, entries[0].Line, "onelinelog.log")
		}, 2*time.Second, 50*time.Millisecond)

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			pos, err := positionsFile.Get(filename, labels.String())
			require.NoError(c, err)
			require.Equal(c, int64(10240), pos)
		}, time.Second, 50*time.Millisecond)

		// Run the decompressor again
		tailer.Run(t.Context())

		// No new entries because the position was kept.
		require.Len(t, received(), 1)
	})
}

func TestTailer_GigantiqueGunzipFile(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
		file := "testdata/long-access.gz"

		tailer := newTailer(
			newMetrics(prometheus.NewRegistry()),
			logging.NewSlogNop(),
			newBatcher,
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
			require.Equal(c, 100000, len(received()))
		}, 2*time.Second, 50*time.Millisecond)
	})
}

// TestTailer_CompressedOnelineFile test the supported formats for log lines that only contain 1 line.
// Based on our experience, this is the scenario with the most edge cases.
func TestTailer_CompressedOnelineFile(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	fileContent, err := os.ReadFile("testdata/onelinelog.log")
	require.NoError(t, err)
	t.Run("gunzip file", func(t *testing.T) {
		runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
			file := "testdata/onelinelog.log.gz"

			tailer := newTailer(
				newMetrics(prometheus.NewRegistry()),
				logging.NewSlogNop(),
				newBatcher,
				positions.NewNop(),
				func() bool { return false },
				sourceOptions{
					path:                file,
					decompressionConfig: DecompressionConfig{Enabled: true, Format: "gz"},
				},
			)

			// We expect tailer to exit when all compressed data have been consumed.
			tailer.Run(t.Context())

			require.Eventually(t, func() bool {
				return len(received()) == 1
			}, 2*time.Second, 50*time.Millisecond)

			entries := received()
			require.Equal(t, string(fileContent), entries[0].Line)
		})
	})

	t.Run("bzip2 file", func(t *testing.T) {
		runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
			file := "testdata/onelinelog.log.bz2"

			tailer := newTailer(
				newMetrics(prometheus.NewRegistry()),
				logging.NewSlogNop(),
				newBatcher,
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
				return len(received()) == 1
			}, 2*time.Second, 50*time.Millisecond)

			entries := received()
			require.Equal(t, string(fileContent), entries[0].Line)
		})
	})

	t.Run("tar.gz file", func(t *testing.T) {
		runTailerTest(t, func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry) {
			file := "testdata/onelinelog.tar.gz"

			tailer := newTailer(
				newMetrics(prometheus.NewRegistry()),
				logging.NewSlogNop(),
				newBatcher,
				positions.NewNop(),
				func() bool { return false },
				sourceOptions{
					path:                file,
					decompressionConfig: DecompressionConfig{Enabled: true, Format: "gz"},
				},
			)

			// We expect tailer to exit when all compressed data have been consumed.
			tailer.Run(t.Context())

			require.Eventually(t, func() bool {
				return len(received()) == 1
			}, 2*time.Second, 50*time.Millisecond)

			entries := received()
			require.Contains(t, entries[0].Line, "onelinelog.log") // contains .tar.gz headers
			require.Contains(
				t,
				entries[0].Line,
				`5.202.214.160 - - [26/Jan/2019:19:45:25 +0330] "GET / HTTP/1.1" 200 30975 "https://www.zanbil.ir/" "Mozilla/5.0 (Windows NT 6.2; WOW64; rv:21.0) Gecko/20100101 Firefox/21.0" "-"`,
			)
		})
	})
}

func runTailerTest(t *testing.T, run func(t *testing.T, newBatcher func(model.LabelSet) loki.StreamBatcher, received func() []loki.Entry)) {
	t.Helper()

	t.Run("pipeline v1", func(t *testing.T) {
		handler := loki.NewCollectingHandler()
		defer handler.Stop()
		run(t, channelBatcher(handler.Receiver()), handler.Received)
	})

	t.Run("pipeline v2", func(t *testing.T) {
		consumer := loki.NewCollectingConsumer()
		run(t, consumerBatcher(consumer, 10), consumer.Entries)
	})
}

func channelBatcher(recv loki.LogsReceiver) func(model.LabelSet) loki.StreamBatcher {
	return func(labels model.LabelSet) loki.StreamBatcher {
		return loki.NewChannelStreamBatcher(loki.NewFanout([]loki.LogsReceiver{recv}), labels)
	}
}

func consumerBatcher(c loki.Consumer, size int) func(model.LabelSet) loki.StreamBatcher {
	return func(labels model.LabelSet) loki.StreamBatcher {
		return loki.NewConsumerStreamBatcher(c, labels, size)
	}
}
