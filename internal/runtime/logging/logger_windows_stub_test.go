//go:build !windows

package logging

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggerWithWindowsEventLogStub(t *testing.T) {
	// Create a logger
	logger, err := NewDeferred(io.Discard)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Try to enable Windows Event Log on non-Windows platform
	options := Options{
		Level:           LevelInfo,
		Format:          FormatLogfmt,
		WindowsEventLog: true,
	}

	err = logger.Update(options)
	// Should fail on non-Windows platforms
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}
