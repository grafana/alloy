//go:build windows

package logging

import (
	"io"
	"testing"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/stretchr/testify/require"
)

func TestLoggerWithWindowsEventLog(t *testing.T) {
	// Create a logger with Windows Event Log enabled
	logger, err := NewDeferred(io.Discard)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Update with Windows Event Log enabled
	options := Options{
		Level:           LevelInfo,
		Format:          FormatLogfmt,
		WindowsEventLog: true,
	}

	err = logger.Update(options)
	require.NoError(t, err)

	// Verify Windows Event Log handler is created
	require.True(t, logger.useWindowsEventLog)
	require.NotNil(t, logger.windowsEventLogHandler)

	// Test logging
	err = logger.Log("level", "info", "msg", "test message", "key", "value")
	require.NoError(t, err)

	// Test disabling Windows Event Log
	options.WindowsEventLog = false
	err = logger.Update(options)
	require.NoError(t, err)

	// Verify Windows Event Log handler is cleaned up
	require.False(t, logger.useWindowsEventLog)
	require.Nil(t, logger.windowsEventLogHandler)
}

func TestLoggerWindowsEventLogLevels(t *testing.T) {
	logger, err := NewDeferred(io.Discard)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Enable Windows Event Log with debug level
	options := Options{
		Level:           LevelDebug,
		Format:          FormatLogfmt,
		WindowsEventLog: true,
	}

	err = logger.Update(options)
	require.NoError(t, err)

	// Test different log levels
	testCases := []struct {
		level string
		msg   string
	}{
		{"debug", "debug message"},
		{"info", "info message"},
		{"warn", "warning message"},
		{"error", "error message"},
	}

	for _, tc := range testCases {
		t.Run(tc.level, func(t *testing.T) {
			err := logger.Log("level", tc.level, "msg", tc.msg)
			require.NoError(t, err)
		})
	}
}

func TestLoggerWindowsEventLogWithWriteTo(t *testing.T) {
	// Create a receiver to capture logs sent via write_to
	receiverChannel := make(chan loki.Entry, 100)
	receiver := loki.NewLogsReceiverWithChannel(receiverChannel)

	logger, err := NewDeferred(io.Discard)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Enable Windows Event Log with write_to
	options := Options{
		Level:           LevelInfo,
		Format:          FormatLogfmt,
		WindowsEventLog: true,
		WriteTo:         []loki.LogsReceiver{receiver},
	}

	err = logger.Update(options)
	require.NoError(t, err)

	// Verify both Windows Event Log and write_to are configured
	require.True(t, logger.useWindowsEventLog)
	require.NotNil(t, logger.windowsEventLogHandler)

	// Test logging - should go to both Windows Event Log and write_to
	err = logger.Log("level", "info", "msg", "test message with write_to", "key", "value")
	require.NoError(t, err)

	// Verify that the log was sent to the receiver
	select {
	case entry := <-receiverChannel:
		require.Contains(t, entry.Entry.Line, "test message with write_to")
		require.Contains(t, entry.Entry.Line, "key=value")
	default:
		t.Fatal("Expected log entry to be sent to write_to receiver")
	}
}
