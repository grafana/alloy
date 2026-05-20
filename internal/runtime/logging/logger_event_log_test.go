package logging

import (
	"bytes"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging/eventlog"
	"github.com/grafana/alloy/internal/runtime/logging/eventlog/testutil"
	"github.com/stretchr/testify/require"
)

// TestLogger_EventLog_LevelReloadTakesEffect verifies that changing the
// log level on an event_log → event_log reload actually propagates to
// the event-log dispatch path. The event log doesn't have its own slog
// handler anymore — formatting and level filtering both go through the
// shared handler — so this exercises that the handler's leveler is
// the live one Update mutates.
func TestLogger_EventLog_LevelReloadTakesEffect(t *testing.T) {
	mock := &testutil.MockEventLog{}
	var inner bytes.Buffer
	l, err := NewDeferred(&inner)
	require.NoError(t, err)
	l.eventLogOpener = func(_ string) (eventlog.EventLog, error) {
		return mock, nil
	}

	// Start at Info level on event_log destination.
	require.NoError(t, l.Update(Options{
		Level:       LevelInfo,
		Format:      FormatLogfmt,
		Destination: LogDestinationWindowsEventLog,
	}))

	sl := l.Slog()
	ctx := t.Context()

	// Debug record should be filtered at Info level.
	require.False(t, sl.Handler().Enabled(ctx, slog.LevelDebug),
		"handler should report debug as disabled at Info level")
	sl.Log(ctx, slog.LevelDebug, "debug-before")
	require.Empty(t, mock.Infos, "debug record at Info level should be filtered")

	// Reload at Debug level, same destination — no reopen.
	require.NoError(t, l.Update(Options{
		Level:       LevelDebug,
		Format:      FormatLogfmt,
		Destination: LogDestinationWindowsEventLog,
	}))

	// Debug record should now pass through.
	require.True(t, sl.Handler().Enabled(ctx, slog.LevelDebug),
		"handler should report debug as enabled at Debug level")
	sl.Log(ctx, slog.LevelDebug, "debug-after")
	require.Len(t, mock.Infos, 1, "debug record at Debug level should reach event log")
	require.Contains(t, mock.Infos[0], "debug-after")
}

// TestLogger_EventLog_RespectsFormatChoice is the whole point of routing
// the event log through the shared handler: the message that lands in the
// Windows Event Log is the same formatted line that goes to stderr/
// write_to, so logfmt → logfmt, json → json. Operators with log
// forwarders parsing the event log get parseable output.
func TestLogger_EventLog_RespectsFormatChoice(t *testing.T) {
	tests := []struct {
		name   string
		format Format
		// expectations on the event log message body
		mustContain []string
	}{
		{
			name:        "logfmt",
			format:      FormatLogfmt,
			mustContain: []string{"level=info", "msg=hello", "k=v"},
		},
		{
			name:        "json",
			format:      FormatJSON,
			mustContain: []string{`"level":"info"`, `"msg":"hello"`, `"k":"v"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &testutil.MockEventLog{}
			var inner bytes.Buffer
			l, err := NewDeferred(&inner)
			require.NoError(t, err)
			l.eventLogOpener = func(_ string) (eventlog.EventLog, error) {
				return mock, nil
			}

			require.NoError(t, l.Update(Options{
				Level:       LevelInfo,
				Format:      tc.format,
				Destination: LogDestinationWindowsEventLog,
			}))

			require.NoError(t, l.Log("msg", "hello", "k", "v"))

			require.Len(t, mock.Infos, 1)
			got := mock.Infos[0]
			for _, want := range tc.mustContain {
				require.Contains(t, got, want, "event log entry should be %s-formatted", tc.format)
			}
			// Event log message shouldn't have the trailing newline slog
			// would emit for stdio output — it's a single API message.
			require.NotContains(t, got, "\n", "event log message should be a single line")
		})
	}
}

// TestLogger_EventLog_TransitionToStderrClosesHandle verifies that
// genuinely leaving the windows_event_log destination DOES close the
// handle and subsequent logs no longer reach it. The bytes path then
// reaches the stderr writer through writerVar's inner writer.
func TestLogger_EventLog_TransitionToStderrClosesHandle(t *testing.T) {
	mock := &testutil.MockEventLog{}
	var inner bytes.Buffer
	l, err := NewDeferred(&inner)
	require.NoError(t, err)
	l.eventLogOpener = func(_ string) (eventlog.EventLog, error) {
		return mock, nil
	}

	require.NoError(t, l.Update(Options{
		Level:       LevelInfo,
		Format:      FormatLogfmt,
		Destination: LogDestinationWindowsEventLog,
	}))

	require.NoError(t, l.Update(Options{
		Level:       LevelInfo,
		Format:      FormatLogfmt,
		Destination: LogDestinationStderr,
	}))
	_, hasEL := l.writer.FastPathFlags()
	require.False(t, hasEL, "event_log → stderr should close the event log handle")

	mock.Reset()
	require.NoError(t, l.Log("msg", "stderr-only"))
	require.Empty(t, mock.Infos, "no further records should reach the event log after transition")
	require.Contains(t, inner.String(), "stderr-only",
		"bytes path should now reach the stderr writer")
}

// TestUpdate_NoLossDuringConcurrentDestinationFlips is a stress test that runs
// many Handle calls in parallel with rapid destination switches between
// stderr and windows_event_log. With Dispatch's loss-prevention fallback AND
// atomic destination transitions (SwitchToEventLog / SwitchToInnerOnly), every
// record lands on exactly one of inner or the event log: never dropped, never
// duplicated. We assert strict equality.
func TestUpdate_NoLossDuringConcurrentDestinationFlips(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}

	const (
		hammerGoroutines = 4
		hammerIters      = 2000
		flipIters        = 200
	)

	mock := &testutil.MockEventLog{}
	inner := &countingWriter{}
	l, err := NewDeferred(inner)
	require.NoError(t, err)
	l.eventLogOpener = func(_ string) (eventlog.EventLog, error) {
		// Re-use the same mock across reopens so cumulative counts make sense.
		return mock, nil
	}

	require.NoError(t, l.Update(Options{
		Level:       LevelInfo,
		Format:      FormatLogfmt,
		Destination: LogDestinationStderr,
	}))

	sl := l.Slog()
	var totalHandles atomic.Int64

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Hammer Handle from N goroutines.
	for i := 0; i < hammerGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < hammerIters; j++ {
				select {
				case <-stop:
					return
				default:
				}
				sl.Info("hammer", "g", id, "j", j)
				totalHandles.Add(1)
				runtime.Gosched()
			}
		}(i)
	}

	// Flip destination between default and event_log.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < flipIters; i++ {
			dest := LogDestinationStderr
			if i%2 == 0 {
				dest = LogDestinationWindowsEventLog
			}
			err := l.Update(Options{
				Level:       LevelInfo,
				Format:      FormatLogfmt,
				Destination: dest,
			})
			if err != nil {
				t.Errorf("Update failed: %v", err)
				close(stop)
				return
			}
			time.Sleep(50 * time.Microsecond)
		}
	}()

	// Bound the test so a hang doesn't wedge CI.
	doneAll := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneAll)
	}()
	select {
	case <-doneAll:
	case <-time.After(30 * time.Second):
		close(stop)
		t.Fatal("stress test timed out")
	}

	// End the test in the default destination so the final accounting is
	// stable.
	require.NoError(t, l.Update(Options{
		Level:       LevelInfo,
		Format:      FormatLogfmt,
		Destination: LogDestinationStderr,
	}))

	innerLines := inner.Lines()
	// Safe to read directly: the goroutines have all returned (wg.Wait) and
	// the final Update has settled, so no concurrent writes to the mock.
	eventLogCount := len(mock.Infos) + len(mock.Warnings) + len(mock.Errors)

	got := int64(innerLines + eventLogCount)
	want := totalHandles.Load()
	require.Equal(t, want, got,
		"each handle should deliver exactly once: handles=%d inner=%d eventlog=%d (sum=%d)",
		want, innerLines, eventLogCount, got)
}

// countingWriter is an io.Writer that records every byte and counts complete
// lines (terminated by '\n'). It mirrors a tracking stderr so the test can
// reconcile delivered records against handle counts.
type countingWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	lines int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			c.lines++
		}
	}
	return c.buf.Write(p)
}

func (c *countingWriter) Lines() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lines
}

func (c *countingWriter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}
