package logging

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	var buf bytes.Buffer
	handler := getTestHandler(t, &buf)
	handler.Handle(context.Background(), newTestRecord("hello world"))

	expect := `level=info msg="hello world"` + "\n"
	require.Equal(t, expect, buf.String())
}

func TestGroups(t *testing.T) {
	var buf bytes.Buffer
	handler := getTestHandler(t, &buf)
	handler = handler.WithAttrs([]slog.Attr{
		slog.String("foo", "bar"),
	})

	handler = handler.WithGroup("test")
	handler = handler.WithAttrs([]slog.Attr{
		slog.String("location", "home"),
	})

	handler = handler.WithGroup("inner")
	handler = handler.WithAttrs([]slog.Attr{
		slog.String("genre", "jazz"),
	})

	handler.Handle(context.Background(), newTestRecord("hello world"))

	expect := `level=info msg="hello world" foo=bar test.location=home test.inner.genre=jazz` + "\n"
	require.Equal(t, expect, buf.String())
}

func newTestRecord(msg string) slog.Record {
	return slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
}

func getTestHandler(t *testing.T, w io.Writer) slog.Handler {
	t.Helper()

	l, err := New(w, Options{
		Level:  LevelDebug,
		Format: FormatLogfmt,
	})
	require.NoError(t, err)

	return l.handler
}
