package logging_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

const testStr = "this is a test string"

func TestLevels(t *testing.T) {
	type testCase struct {
		name            string
		configuredLevel logging.Level
		level           slog.Level
		message         string
		expected        string
	}

	var testCases = []testCase{

		{
			name:            "debug - prints",
			configuredLevel: logging.LevelDebug,
			level:           slog.LevelDebug,
			message:         "hello",
			expected:        "level=debug msg=hello\n",
		},
		{
			name:            "debug - drops",
			configuredLevel: logging.LevelInfo,
			level:           slog.LevelDebug,
			message:         "hello",
			expected:        "",
		},

		{
			name:            "info - drops",
			configuredLevel: logging.LevelWarn,
			level:           slog.LevelInfo,
			message:         "hello",
			expected:        "",
		},
		{
			name:            "level - prints",
			configuredLevel: logging.LevelInfo,
			level:           slog.LevelInfo,

			message:  "hello",
			expected: "level=info msg=hello\n",
		},
		{
			name:            "warn - drops",
			configuredLevel: logging.LevelError,
			level:           slog.LevelWarn,
			message:         "hello",
			expected:        "",
		},

		{
			name:            "warn - prints",
			configuredLevel: logging.LevelInfo,
			level:           slog.LevelWarn,
			message:         "hello",
			expected:        "level=warn msg=hello\n",
		},
		{
			name:            "error - prints",
			configuredLevel: logging.LevelError,
			level:           slog.LevelError,
			message:         "hello",
			expected:        "level=error msg=hello\n",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			buffer := bytes.NewBuffer(nil)
			opts := logging.Options{}
			opts.SetToDefault()
			opts.Level = tt.configuredLevel
			logger, err := logging.New(buffer, opts)
			require.NoError(t, err)
			logger.Slog().Log(context.Background(), tt.level, tt.message)
			if tt.expected == "" {
				require.Empty(t, buffer.String())
			} else {
				require.Contains(t, buffer.String(), "ts=")
				noTimestamp := strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
				require.Equal(t, tt.expected, noTimestamp)
			}
		})
	}
}

// TestUpdateRoutesLogs walks the logger through every combination of
// destination (stderr/none) and write_to (zero/one/many receivers), and for
// each step verifies that exactly the expected sinks observe the log.
func TestUpdateRoutesLogs(t *testing.T) {
	const (
		// arrivalTimeout bounds how long we wait for a sink we *expect* to
		// receive a log.
		arrivalTimeout = time.Second
		// arrivalPollInterval is how often we re-check the expected sink
		// while waiting.
		arrivalPollInterval = 10 * time.Millisecond
		// absenceWait is how long we let any (incorrect) write land before
		// asserting that a sink we do *not* expect remained empty.
		absenceWait = 100 * time.Millisecond
	)

	var buf safeBuffer
	receivers := []loki.LogsReceiver{
		loki.NewLogsReceiver(loki.WithChannel(make(chan loki.Entry, 16))),
		loki.NewLogsReceiver(loki.WithChannel(make(chan loki.Entry, 16))),
	}

	logger, err := logging.NewDeferred(&buf)
	require.NoError(t, err)

	steps := []struct {
		name          string
		destination   logging.LogDestination
		writeTo       []int // indices into receivers
		wantStderr    bool
		wantReceivers []int // indices into receivers
	}{
		{
			name:        "destination=none, no write_to: everything dropped",
			destination: logging.LogDestinationNone,
		},
		{
			name:          "destination=none with write_to: only receiver gets the log",
			destination:   logging.LogDestinationNone,
			writeTo:       []int{0},
			wantReceivers: []int{0},
		},
		{
			name:          "destination=stderr with write_to: stderr and receiver get the log",
			destination:   logging.LogDestinationStderr,
			writeTo:       []int{0},
			wantStderr:    true,
			wantReceivers: []int{0},
		},
		{
			name:          "destination=stderr with multiple receivers: stderr and all receivers get the log",
			destination:   logging.LogDestinationStderr,
			writeTo:       []int{0, 1},
			wantStderr:    true,
			wantReceivers: []int{0, 1},
		},
		{
			name:        "destination=stderr, write_to cleared: only stderr gets the log",
			destination: logging.LogDestinationStderr,
			wantStderr:  true,
		},
		{
			name:        "destination=none, write_to cleared: everything dropped",
			destination: logging.LogDestinationNone,
		},
	}

	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			var writeTo []loki.LogsReceiver
			for _, idx := range step.writeTo {
				writeTo = append(writeTo, receivers[idx])
			}
			require.NoError(t, logger.Update(logging.Options{
				Level:       logging.LevelDebug,
				Format:      logging.FormatLogfmt,
				Destination: step.destination,
				WriteTo:     writeTo,
			}))

			msg := fmt.Sprintf("step-%d", i)
			beforeBuf := buf.String()
			require.NoError(t, logger.Log("msg", msg))

			if step.wantStderr {
				require.Eventually(t, func() bool {
					return strings.Contains(buf.String(), msg)
				}, arrivalTimeout, arrivalPollInterval, "stderr did not receive %q", msg)
			} else {
				time.Sleep(absenceWait)
				require.NotContains(t, strings.TrimPrefix(buf.String(), beforeBuf), msg,
					"stderr unexpectedly received %q", msg)
			}

			wantSet := make(map[int]bool, len(step.wantReceivers))
			for _, idx := range step.wantReceivers {
				wantSet[idx] = true
			}
			for idx, r := range receivers {
				if wantSet[idx] {
					select {
					case entry := <-r.Chan():
						require.Contains(t, entry.Line, msg,
							"receiver %d got wrong log", idx)
					case <-time.After(arrivalTimeout):
						t.Fatalf("receiver %d did not receive %q", idx, msg)
					}
				} else {
					select {
					case entry := <-r.Chan():
						t.Fatalf("receiver %d unexpectedly got %q", idx, entry.Line)
					case <-time.After(absenceWait):
					}
				}
			}
		})
	}
}

// Test_lokiWriter_nil ensures that writing to a lokiWriter doesn't panic when
// given a nil receiver.
func Test_lokiWriter_nil(t *testing.T) {
	logger, err := logging.New(io.Discard, debugLevel())
	require.NoError(t, err)

	err = logger.Update(logging.Options{
		Level:       logging.LevelDebug,
		Format:      logging.FormatLogfmt,
		Destination: logging.LogDestinationStderr,

		WriteTo: []loki.LogsReceiver{nil},
	})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		logger.Slog().Info("test message")
	})
}

// TestWriteToDisabledViaUpdate verifies that logs go to both stderr and the
// configured write_to receiver while write_to is set, and that calling Update
// with an empty WriteTo stops sending logs to the receiver while still emitting
// to stderr.
func TestWriteToDisabledViaUpdate(t *testing.T) {
	var buf safeBuffer
	receiver := loki.NewLogsReceiver(loki.WithChannel(make(chan loki.Entry, 16)))

	logger, err := logging.New(&buf, debugLevel())
	require.NoError(t, err)

	require.NoError(t, logger.Update(logging.Options{
		Level:       logging.LevelDebug,
		Format:      logging.FormatLogfmt,
		Destination: logging.LogDestinationStderr,
		WriteTo:     []loki.LogsReceiver{receiver},
	}))

	slogger := logger.Slog()
	slogger.Info("with-write-to")

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "with-write-to")
	}, time.Second, 10*time.Millisecond, "stderr did not receive log while write_to was enabled")

	select {
	case entry := <-receiver.Chan():
		require.Contains(t, entry.Line, "with-write-to")
	case <-time.After(time.Second):
		t.Fatal("write_to receiver did not receive log while write_to was enabled")
	}

	require.NoError(t, logger.Update(logging.Options{
		Level:       logging.LevelDebug,
		Format:      logging.FormatLogfmt,
		Destination: logging.LogDestinationStderr,
	}))

	beforeLen := buf.String()
	slogger.Info("without-write-to")

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "without-write-to")
	}, time.Second, 10*time.Millisecond, "stderr did not receive log after write_to was disabled")
	require.Greater(t, len(buf.String()), len(beforeLen))

	select {
	case entry := <-receiver.Chan():
		t.Fatalf("write_to receiver got log %q after write_to was disabled", entry.Line)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestUpdateConcurrentHandle verifies that Logger.Update completes successfully
// when child handlers are being used concurrently, as happens when components
// start logging during graph evaluation before the logging config block is processed.
func TestUpdateConcurrentHandle(t *testing.T) {
	l, err := logging.NewDeferred(io.Discard)
	require.NoError(t, err)

	child := l.Handler().WithAttrs([]slog.Attr{slog.String("component", "test")})

	stop := make(chan struct{})
	defer close(stop)

	go func() {
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, "concurrent log", 0)
		for {
			select {
			case <-stop:
				return
			default:
				_ = child.Handle(context.Background(), rec)
				runtime.Gosched()
			}
		}
	}()

	done := make(chan error, 1)
	go func() {
		done <- l.Update(logging.Options{
			Level:       logging.LevelInfo,
			Format:      logging.FormatLogfmt,
			Destination: logging.LogDestinationStderr,
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Logger.Update did not complete while child handlers were being used concurrently")
	}
}

// TestUpdateNoLostBufferedMessages verifies that log records buffered before
// Update is called are not lost even when concurrent Handle calls are in-flight
// during the window between bufferMut being released and buildHandlers completing.
func TestUpdateNoLostBufferedMessages(t *testing.T) {
	var buf safeBuffer
	l, err := logging.NewDeferred(&buf)
	require.NoError(t, err)

	// Buffer some messages before Update is called.
	child := l.Handler().WithAttrs([]slog.Attr{slog.String("component", "test")})
	for i := range 10 {
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf("buffered-%d", i), 0)
		require.NoError(t, child.Handle(context.Background(), rec))
	}

	// Hammer Handle from a separate goroutine so that calls are in-flight while
	// Update releases bufferMut to run buildHandlers.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, "concurrent", 0)
		for {
			select {
			case <-stop:
				return
			default:
				_ = child.Handle(context.Background(), rec)
				runtime.Gosched()
			}
		}
	}()

	require.NoError(t, l.Update(logging.Options{
		Level:       logging.LevelInfo,
		Format:      logging.FormatLogfmt,
		Destination: logging.LogDestinationStderr,
	}))
	close(stop)
	<-done

	for i := range 10 {
		require.Contains(t, buf.String(), fmt.Sprintf("buffered-%d", i), "buffered message %d was lost", i)
	}
}

// TestHandlerAfterUpdateIsReal verifies that a child handler created via
// WithAttrs or WithGroup after Update has been called writes directly to the
// underlying handler rather than buffering.
func TestHandlerAfterUpdateIsReal(t *testing.T) {
	var buf bytes.Buffer
	l, err := logging.New(&buf, infoLevel())
	require.NoError(t, err)

	// Handler created after Update should write immediately — no second Update needed.
	child := l.Handler().WithAttrs([]slog.Attr{slog.String("component", "post-update")})
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "direct-write", 0)
	require.NoError(t, child.Handle(context.Background(), rec))

	require.Contains(t, buf.String(), "direct-write")
}

func BenchmarkLogging_Slog_Drops(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		logger.Debug("test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
}

func BenchmarkLogging_Slog_Prints(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		logger.Info("test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
}

func debugLevel() logging.Options {
	opts := logging.Options{}
	opts.SetToDefault()
	opts.Level = logging.LevelDebug
	return opts
}

func infoLevel() logging.Options {
	opts := debugLevel()
	opts.Level = logging.LevelInfo
	return opts
}
