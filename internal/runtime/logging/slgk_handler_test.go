package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

// TestEnabledWithWrappedLogger verifies that SlogGoKitHandler.Enabled() correctly
// reports the configured level when the go-kit logger is wrapped via log.With().
// This is important for libraries like the Percona mongodb_exporter that call
// Enabled() to decide whether to emit output via non-slog paths.
func TestEnabledWithWrappedLogger(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	baseLogger, err := logging.New(buffer, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	// Wrap like the component runtime does: log.With strips EnabledAware,
	// so use NewLevelAwareLogger to restore it.
	wrapped := logging.NewLevelAwareLogger(log.With(baseLogger, "component", "test"), baseLogger)
	handler := logging.NewSlogGoKitHandler(wrapped)

	ctx := context.Background()
	require.False(t, handler.Enabled(ctx, slog.LevelDebug), "debug should be disabled at info level")
	require.True(t, handler.Enabled(ctx, slog.LevelInfo), "info should be enabled at info level")
	require.True(t, handler.Enabled(ctx, slog.LevelError), "error should be enabled at info level")

	err = baseLogger.Update(logging.Options{Level: logging.LevelError, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	require.False(t, handler.Enabled(ctx, slog.LevelDebug), "debug should be disabled at error level")
	require.False(t, handler.Enabled(ctx, slog.LevelInfo), "info should be disabled at error level")
	require.True(t, handler.Enabled(ctx, slog.LevelError), "error should be enabled at error level")
}

func TestUpdateLevel(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	baseLogger, err := logging.New(buffer, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	gkLogger := log.With(baseLogger, "test", "test")
	gkLogger.Log("msg", "hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp := strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=info msg=hello test=test\n", noTimestamp)

	sLogger := slog.New(logging.NewSlogGoKitHandler(gkLogger))
	buffer.Reset()
	sLogger.Info("hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp = strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=info msg=hello test=test\n", noTimestamp)

	buffer.Reset()
	sLogger.Debug("hello")
	require.Equal(t, "", buffer.String())

	err = baseLogger.Update(logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	buffer.Reset()
	sLogger.Info("hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp = strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=info msg=hello test=test\n", noTimestamp)

	buffer.Reset()
	sLogger.Debug("hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp = strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=debug msg=hello test=test\n", noTimestamp)
}
