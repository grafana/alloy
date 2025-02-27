package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"testing/slogtest"
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

func TestSlogTester(t *testing.T) {
	var buf bytes.Buffer
	s, err := NewService(&buf)
	require.NoError(t, err)
	err = s.Update(Options{
		Level:  "debug",
		Format: "json",
	})
	require.NoError(t, err)
	l := s.Data().(*Logger)
	require.NoError(t, err)
	results := func() []map[string]any {
		var ms []map[string]any
		for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var m map[string]any
			unmarshalErr := json.Unmarshal(line, &m)
			require.NoError(t, unmarshalErr)
			// The tests expect time field and not ts.
			if _, found := m["ts"]; found {
				m[slog.TimeKey] = m["ts"]
				delete(m, "ts")
			}
			ms = append(ms, m)
		}
		return ms
	}
	err = slogtest.TestHandler(l.handler, results)
	require.NoError(t, err)
}

func newTestRecord(msg string) slog.Record {
	return slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
}

func getTestHandler(t *testing.T, w io.Writer) slog.Handler {
	t.Helper()

	s, err := NewService(w)
	require.NoError(t, err)
	err = s.Update(Options{
		Level:  LevelDebug,
		Format: FormatLogfmt,
	})
	require.NoError(t, err)
	l := s.Data().(*Logger)

	return l.handler
}

// testReplace is used for unit tests so we can ensure the time and source fields are consistent.
func testReplace(groups []string, a slog.Attr) slog.Attr {
	ra := replace(groups, a)
	switch a.Key {
	case "ts":
		fallthrough
	case "time":
		return slog.Attr{
			Key:   "ts",
			Value: slog.StringValue("2024-04-29T18:26:21.37723798Z"),
		}
	case "source":
		return slog.Attr{
			Key:   "source",
			Value: slog.StringValue("test_source"),
		}
	default:
		return ra
	}
}

// newDeferredTest creates a new logging service with the default log level and format. Used for tests.
// The logger is not updated during initialization.
func newDeferredTest(w io.Writer) (*Service, error) {
	s, err := NewService(w)
	if err != nil {
		return nil, err
	}
	l := s.Data().(*Logger)
	l.handler.replacer = testReplace

	return s, nil
}
