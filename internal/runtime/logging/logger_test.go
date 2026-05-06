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

	"github.com/go-kit/log"
	gokitlevel "github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
	alloylevel "github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/stretchr/testify/require"
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

/* Most recent performance results on M2 Macbook Air:
$ go test -count=1 -benchmem ./internal/alloy/logging -run ^$ -bench BenchmarkLogging_
goos: darwin
goarch: arm64
pkg: github.com/grafana/alloy/internal/runtime/logging
BenchmarkLogging_NoLevel_Prints-8             	  722358	      1524 ns/op	     368 B/op	      11 allocs/op
BenchmarkLogging_NoLevel_Drops-8              	47103154	        25.59 ns/op	       8 B/op	       0 allocs/op
BenchmarkLogging_GoKitLevel_Drops_Sprintf-8   	 3585387	       332.1 ns/op	     320 B/op	       8 allocs/op
BenchmarkLogging_GoKitLevel_Drops-8           	 6705489	       176.6 ns/op	     472 B/op	       5 allocs/op
BenchmarkLogging_GoKitLevel_Prints-8          	  678214	      1669 ns/op	     849 B/op	      16 allocs/op
BenchmarkLogging_Slog_Drops-8                 	79687671	        15.09 ns/op	       8 B/op	       0 allocs/op
BenchmarkLogging_Slog_Prints-8                	 1000000	      1119 ns/op	      32 B/op	       2 allocs/op
BenchmarkLogging_AlloyLevel_Drops-8            	21693330	        58.45 ns/op	     168 B/op	       2 allocs/op
BenchmarkLogging_AlloyLevel_Prints-8           	  720554	      1672 ns/op	     833 B/op	      15 allocs/op
*/

const testStr = "this is a test string"

func TestLevels(t *testing.T) {
	type testCase struct {
		name     string
		logger   func(w io.Writer) (log.Logger, error)
		message  string
		expected string
	}

	var testCases = []testCase{
		{
			name:     "no level - prints",
			logger:   func(w io.Writer) (log.Logger, error) { return logging.New(w, debugLevel()) },
			message:  "hello",
			expected: "level=info msg=hello\n",
		},
		{
			name:     "no level - drops",
			logger:   func(w io.Writer) (log.Logger, error) { return logging.New(w, warnLevel()) },
			message:  "hello",
			expected: "",
		},
		{
			name: "alloy info level - drops",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, warnLevel())
				return alloylevel.Info(logger), err
			},
			message:  "hello",
			expected: "",
		},
		{
			name: "alloy debug level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, debugLevel())
				return alloylevel.Debug(logger), err
			},
			message:  "hello",
			expected: "level=debug msg=hello\n",
		},
		{
			name: "alloy info level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, infoLevel())
				return alloylevel.Info(logger), err
			},
			message:  "hello",
			expected: "level=info msg=hello\n",
		},
		{
			name: "alloy warn level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, debugLevel())
				return alloylevel.Warn(logger), err
			},
			message:  "hello",
			expected: "level=warn msg=hello\n",
		},
		{
			name: "alloy error level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, debugLevel())
				return alloylevel.Error(logger), err
			},
			message:  "hello",
			expected: "level=error msg=hello\n",
		},
		{
			name: "gokit info level - drops",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, warnLevel())
				return gokitlevel.Info(logger), err
			},
			message:  "hello",
			expected: "",
		},
		{
			name: "gokit debug level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, debugLevel())
				return gokitlevel.Debug(logger), err
			},
			message:  "hello",
			expected: "level=debug msg=hello\n",
		},
		{
			name: "gokit info level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, infoLevel())
				return gokitlevel.Info(logger), err
			},
			message:  "hello",
			expected: "level=info msg=hello\n",
		},
		{
			name: "gokit warn level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, debugLevel())
				return gokitlevel.Warn(logger), err
			},
			message:  "hello",
			expected: "level=warn msg=hello\n",
		},
		{
			name: "gokit error level - prints",
			logger: func(w io.Writer) (log.Logger, error) {
				logger, err := logging.New(w, debugLevel())
				return gokitlevel.Error(logger), err
			},
			message:  "hello",
			expected: "level=error msg=hello\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buffer := bytes.NewBuffer(nil)
			logger, err := tc.logger(buffer)
			require.NoError(t, err)
			logger.Log("msg", tc.message)

			if tc.expected == "" {
				require.Empty(t, buffer.String())
			} else {
				require.Contains(t, buffer.String(), "ts=")
				noTimestamp := strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
				require.Equal(t, tc.expected, noTimestamp)
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
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,

		WriteTo: []loki.LogsReceiver{nil},
	})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_ = logger.Log("msg", "test message")
	})
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
		done <- l.Update(logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
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

	require.NoError(t, l.Update(logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt}))
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

func BenchmarkLogging_NoLevel_Prints(b *testing.B) {
	logger, err := logging.New(io.Discard, infoLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		logger.Log("msg", "test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
}

func BenchmarkLogging_NoLevel_Drops(b *testing.B) {
	logger, err := logging.New(io.Discard, warnLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		logger.Log("msg", "test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
}

func BenchmarkLogging_GoKitLevel_Drops_Sprintf(b *testing.B) {
	logger, err := logging.New(io.Discard, infoLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		gokitlevel.Debug(logger).Log("msg", fmt.Sprintf("test message %d, error=%v, str=%s, duration=%v", i, testErr, testStr, time.Second))
	}
}

func BenchmarkLogging_GoKitLevel_Drops(b *testing.B) {
	logger, err := logging.New(io.Discard, infoLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		gokitlevel.Debug(logger).Log("msg", "test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
}

func BenchmarkLogging_GoKitLevel_Prints(b *testing.B) {
	logger, err := logging.New(io.Discard, infoLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	testStr := "this is a test string"
	for i := 0; i < b.N; i++ {
		gokitlevel.Warn(logger).Log("msg", "test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
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

func BenchmarkLogging_AlloyLevel_Drops(b *testing.B) {
	logger, err := logging.New(io.Discard, infoLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		alloylevel.Debug(logger).Log("msg", "test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
	}
}

func BenchmarkLogging_AlloyLevel_Prints(b *testing.B) {
	logger, err := logging.New(io.Discard, infoLevel())
	require.NoError(b, err)

	testErr := fmt.Errorf("test error")
	for i := 0; i < b.N; i++ {
		alloylevel.Info(logger).Log("msg", "test message", "i", i, "err", testErr, "str", testStr, "duration", time.Second)
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

func warnLevel() logging.Options {
	opts := debugLevel()
	opts.Level = logging.LevelWarn
	return opts
}
