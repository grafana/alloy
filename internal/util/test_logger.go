package util

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

// TestLogger generates a logger for a test.
func TestLogger(t testing.TB) *slog.Logger {
	t.Helper()

	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.String("ts", a.Value.Time().UTC().Format("15:04:05.000"))
			}
			return a
		},
	}))

	return l.With("test", t.Name())
}

// TestAlloyLogger generates an Alloy-compatible logger for a test.
func TestAlloyLogger(t require.TestingT) *logging.Logger {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}

	l, err := logging.New(os.Stderr, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)
	return l
}
