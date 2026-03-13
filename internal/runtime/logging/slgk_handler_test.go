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

// enabledAwareLogger is a test wrapper that implements both log.Logger and
// logging.EnabledAware, simulating what mongodb_exporter's levelAwareLogger does.
type enabledAwareLogger struct {
	log.Logger
	minLevel slog.Level
}

func (l *enabledAwareLogger) Enabled(_ context.Context, level slog.Level) bool {
	return level >= l.minLevel
}

func TestEnabledAwareLogger(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	baseLogger, err := logging.New(buffer, logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	// Wrap with EnabledAware that only allows info and above.
	wrapper := &enabledAwareLogger{
		Logger:   baseLogger,
		minLevel: slog.LevelInfo,
	}

	sLogger := slog.New(logging.NewSlogGoKitHandler(wrapper))

	// Debug should be suppressed because EnabledAware reports false for debug.
	require.False(t, sLogger.Enabled(context.Background(), slog.LevelDebug))

	// Info should be allowed.
	require.True(t, sLogger.Enabled(context.Background(), slog.LevelInfo))

	// Warn and Error should be allowed.
	require.True(t, sLogger.Enabled(context.Background(), slog.LevelWarn))
	require.True(t, sLogger.Enabled(context.Background(), slog.LevelError))
}

func TestEnabledFallbackWithoutEnabledAware(t *testing.T) {
	// A plain go-kit logger that does NOT implement EnabledAware should
	// cause Enabled() to return true (the safe fallback).
	plainLogger := log.NewNopLogger()
	sLogger := slog.New(logging.NewSlogGoKitHandler(plainLogger))

	// All levels should return true because the fallback is permissive.
	require.True(t, sLogger.Enabled(context.Background(), slog.LevelDebug))
	require.True(t, sLogger.Enabled(context.Background(), slog.LevelInfo))
}
