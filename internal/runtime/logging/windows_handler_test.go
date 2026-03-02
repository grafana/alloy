package logging

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging/eventlog/testutil"
	"github.com/stretchr/testify/require"
)

func TestWindowsEventLogHandler(t *testing.T) {
	mock := &testutil.MockEventLog{}
	leveler := &slog.LevelVar{}
	handler := newWindowsEventLogHandler(mock, leveler, replace)
	require.NotNil(t, handler)
	defer handler.Close()

	ctx := context.Background()

	tests := []struct {
		name            string
		levelerLevel    slog.Level
		recordLevel     slog.Level
		message         string
		attrs           []slog.Attr
		expectedMessage string
		expectInfo      bool
		expectWarning   bool
		expectError     bool
	}{
		{
			name:            "debug",
			levelerLevel:    slog.LevelDebug,
			recordLevel:     slog.LevelDebug,
			message:         "debug event",
			expectedMessage: "debug event",
			expectInfo:      true,
		},
		{
			name:            "info",
			levelerLevel:    slog.LevelInfo,
			recordLevel:     slog.LevelInfo,
			message:         "info event",
			expectedMessage: "info event",
			expectInfo:      true,
		},
		{
			name:            "warn",
			levelerLevel:    slog.LevelWarn,
			recordLevel:     slog.LevelWarn,
			message:         "warn event",
			expectedMessage: "warn event",
			expectWarning:   true,
		},
		{
			name:            "error",
			levelerLevel:    slog.LevelError,
			recordLevel:     slog.LevelError,
			message:         "error event",
			expectedMessage: "error event",
			expectError:     true,
		},
		{
			name:            "with_attrs",
			levelerLevel:    slog.LevelInfo,
			recordLevel:     slog.LevelInfo,
			message:         "event with attrs",
			attrs:           []slog.Attr{slog.String("key", "value"), slog.Int("n", 42)},
			expectedMessage: "event with attrs key=value n=42",
			expectInfo:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.Reset()
			leveler.Set(tt.levelerLevel)

			record := slog.NewRecord(time.Now(), tt.recordLevel, tt.message, 0)
			for _, a := range tt.attrs {
				record.AddAttrs(a)
			}

			err := handler.Handle(ctx, record)
			require.NoError(t, err)

			if tt.expectInfo {
				require.Len(t, mock.Infos, 1, "expected one Info call")
				require.Contains(t, mock.Infos[0], tt.expectedMessage)
			}
			if tt.expectWarning {
				require.Len(t, mock.Warnings, 1, "expected one Warning call")
				require.Contains(t, mock.Warnings[0], tt.expectedMessage)
			}
			if tt.expectError {
				require.Len(t, mock.Errors, 1, "expected one Error call")
				require.Contains(t, mock.Errors[0], tt.expectedMessage)
			}
		})
	}
}
