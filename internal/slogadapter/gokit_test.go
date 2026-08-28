package slogadapter

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/go-kit/log/level"
	"github.com/stretchr/testify/require"
)

func TestFiltersLogs(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,

		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Drop timestamps for reproducible tests.
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}

			return a
		},
	})

	l := GoKit(h)
	level.Debug(l).Log("msg", "debug level log")
	level.Info(l).Log("msg", "info level log")
	level.Warn(l).Log("msg", "warn level log")
	level.Error(l).Log("msg", "error level log")

	expect := `level=WARN msg="warn level log"
level=ERROR msg="error level log"
`

	require.Equal(t, expect, buf.String())
}
