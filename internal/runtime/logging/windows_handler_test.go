//go:build windows

package logging

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWindowsEventLogHandler(t *testing.T) {
	// Create a Windows Event Log handler
	leveler := &slog.LevelVar{}
	leveler.Set(slog.LevelInfo)

	handler, err := newWindowsEventLogHandler("AlloyTest", leveler, replace)
	require.NoError(t, err)
	require.NotNil(t, handler)
	defer handler.Close()

	// Test Enabled method
	ctx := context.Background()
	require.True(t, handler.Enabled(ctx, slog.LevelInfo))
	require.True(t, handler.Enabled(ctx, slog.LevelError))
	require.False(t, handler.Enabled(ctx, slog.LevelDebug))

	// Test Handle method with a simple record
	record := slog.NewRecord(
		time.Now(),
		slog.LevelInfo,
		"Test message",
		0,
	)
	record.AddAttrs(slog.String("key", "value"))

	err = handler.Handle(ctx, record)
	require.NoError(t, err)

	// Test WithAttrs
	handlerWithAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("attr1", "value1"),
	})
	require.NotNil(t, handlerWithAttrs)

	// Test WithGroup
	handlerWithGroup := handler.WithGroup("testgroup")
	require.NotNil(t, handlerWithGroup)
}

func TestWindowsEventLogHandlerLogLevels(t *testing.T) {
	leveler := &slog.LevelVar{}
	leveler.Set(slog.LevelDebug)

	handler, err := newWindowsEventLogHandler("AlloyTest", leveler, replace)
	require.NoError(t, err)
	require.NotNil(t, handler)
	defer handler.Close()

	ctx := context.Background()

	// Test different log levels
	levels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			record := slog.NewRecord(
				time.Now(),
				level,
				"Test message for "+level.String(),
				0,
			)

			err := handler.Handle(ctx, record)
			require.NoError(t, err)
		})
	}
}
