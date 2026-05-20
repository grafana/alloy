package logging

import (
	"bytes"
	"log/slog"
	"testing"

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
