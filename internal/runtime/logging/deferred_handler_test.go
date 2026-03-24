package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"github.com/go-kit/log/level"
	"github.com/stretchr/testify/require"
)

func TestDefferredSlogTester(t *testing.T) {
	buf := &bytes.Buffer{}
	var l *Logger
	results := func(t *testing.T) map[string]any {
		// Nothing has been written to the byte stream, it only exists in the internal logger buffer
		// We need to call l.Update to flush it to the byte stream.
		// This does something a bit ugly where it DEPENDS on the var in slogtest.Run, if the behavior of slogtest.Run
		// changes this may break the tests.
		updateErr := l.Update(Options{
			Level:   "debug",
			Format:  "json",
			WriteTo: nil,
		})
		require.NoError(t, updateErr)
		line := buf.Bytes()
		if len(line) == 0 {
			return nil
		}
		var m map[string]any
		unmarshalErr := json.Unmarshal(line, &m)
		require.NoError(t, unmarshalErr)
		// The tests expect time field and not ts.
		if _, found := m["ts"]; found {
			m[slog.TimeKey] = m["ts"]
			delete(m, "ts")
		}
		// Need to reset the buffer and logger between each test.
		l = nil
		buf.Reset()
		return m
	}

	// Had to add some custom logic to handle updated for the deferred tests.
	// Also ignore anything that modifies the log line, which are two tests.
	slogtest.Run(t, func(t *testing.T) slog.Handler {
		var err error
		l, err = NewDeferred(buf)
		require.NoError(t, err)
		return l.Handler()
	}, results)
}

func TestDeferredHandlerWritingToBothLoggers(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := NewDeferred(bb)
	slogger := slog.New(l.deferredSlog)
	require.NoError(t, err)
	l.Log("msg", "this should happen before")
	slogger.Log(t.Context(), slog.LevelInfo, "this should happen after)")

	err = l.Update(Options{
		Level:   "info",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	firstIndex := strings.Index(bb.String(), "this should happen before")
	secondIndex := strings.Index(bb.String(), "this should happen after")
	require.True(t, firstIndex < secondIndex)
}

func TestSlogHandle(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)
	logInfo(t.Context(), sl, alloy, "simple_test")
	err := l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, equal(bb, bbSl))
}

func TestSlogHandleWithDifferingLevelDeny(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)
	logInfo(t.Context(), sl, alloy, "test_denied")
	err := l.Update(Options{
		Level:   "warn",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, bb.Len() == 0)
}

func TestSlogHandleWithDifferingLevelAllow(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)
	logError(t.Context(), sl, alloy, "test3")
	err := l.Update(Options{
		Level:   "warn",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, bb.Len() > 0)
}

func TestNormalWithDifferingLevelDeny(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := newDeferredTest(bb)
	require.NoError(t, err)
	level.Debug(l).Log("msg", "this should not log")
	err = l.Update(Options{
		Level:   "error",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, bb.Len() == 0)
}

func TestNormalWithDifferingLevelAllow(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := newDeferredTest(bb)
	require.NoError(t, err)
	level.Error(l).Log("msg", "this should not log")
	err = l.Update(Options{
		Level:   "error",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	// Since we write logs at info, but change to error then our logInfo should be clean.
	require.True(t, bb.Len() > 0)
}

func TestDeferredHandler(t *testing.T) {
	type testCase struct {
		name string
		log  func(bb *bytes.Buffer, slBB *bytes.Buffer)
	}

	var testCases = []testCase{
		{
			name: "Single Attr",
			log: func(bb *bytes.Buffer, bbSl *bytes.Buffer) {
				sl, alloy, l := newLoggers(t, bb, bbSl)

				sl, alloy = withAttrs(sl, alloy, "attr1", "value1")
				logInfo(t.Context(), sl, alloy, "test_attr")
				err := l.Update(Options{
					Level:   "debug",
					Format:  "json",
					WriteTo: nil,
				})
				require.NoError(t, err)
			},
		},
		{
			name: "Attrs Nested",
			log: func(bb *bytes.Buffer, bbSl *bytes.Buffer) {
				sl, alloy, l := newLoggers(t, bb, bbSl)
				sl, alloy = withAttrs(sl, alloy, "attr1", "value1")
				sl, alloy = withAttrs(sl, alloy, "nestedattr1", "nestedvalue1")
				logInfo(t.Context(), sl, alloy, "test_nested")
				err := l.Update(Options{
					Level:   "debug",
					Format:  "json",
					WriteTo: nil,
				})
				require.NoError(t, err)
			},
		},
		{
			name: "Group",
			log: func(bb *bytes.Buffer, bbSl *bytes.Buffer) {
				sl, alloy, l := newLoggers(t, bb, bbSl)
				sl, alloy = withGroup(sl, alloy, "gr1")
				sl, alloy = withAttrs(sl, alloy, "nestedattr1", "nestedvalue1")
				logInfo(t.Context(), sl, alloy, "test_group")
				err := l.Update(Options{
					Level:   "debug",
					Format:  "json",
					WriteTo: nil,
				})
				require.NoError(t, err)
			},
		},
		{
			name: "Nested Group",
			log: func(bb *bytes.Buffer, bbSl *bytes.Buffer) {
				sl, alloy, l := newLoggers(t, bb, bbSl)
				sl, alloy = withGroup(sl, alloy, "gr1")
				sl, alloy = withGroup(sl, alloy, "gr2")
				sl, alloy = withAttrs(sl, alloy, "nestedattr1", "nestedvalue1")
				logInfo(t.Context(), sl, alloy, "test_nested_group")
				err := l.Update(Options{
					Level:   "debug",
					Format:  "json",
					WriteTo: nil,
				})
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bb := &bytes.Buffer{}
			bbSl := &bytes.Buffer{}
			tc.log(bb, bbSl)
			require.True(t, equal(bb, bbSl))
		})
	}
}

func newLoggers(t *testing.T, bb, bbSl *bytes.Buffer) (*slog.Logger, *slog.Logger, *Logger) {
	l, err := newDeferredTest(bb)
	require.NoError(t, err)

	jsonH := slog.NewJSONHandler(bbSl, &slog.HandlerOptions{
		AddSource:   true,
		Level:       nil,
		ReplaceAttr: testReplace,
	})
	sl := slog.New(jsonH)
	alloy := slog.New(l.deferredSlog)
	return sl, alloy, l
}

func withAttrs(sl *slog.Logger, alloyL *slog.Logger, attrs ...string) (*slog.Logger, *slog.Logger) {
	var attrAny []any
	for _, a := range attrs {
		attrAny = append(attrAny, a)
	}
	return sl.With(attrAny...), alloyL.With(attrAny...)
}

func withGroup(sl *slog.Logger, alloyL *slog.Logger, group string) (*slog.Logger, *slog.Logger) {
	return sl.WithGroup(group), alloyL.WithGroup(group)
}

func logInfo(ctx context.Context, sl *slog.Logger, alloyL *slog.Logger, msg string) {
	sl.Log(ctx, slog.LevelInfo, msg)
	alloyL.Log(ctx, slog.LevelInfo, msg)
}

func logError(ctx context.Context, sl *slog.Logger, alloyL *slog.Logger, msg string) {
	sl.Log(ctx, slog.LevelError, msg)
	alloyL.Log(ctx, slog.LevelError, msg)
}

func equal(sl *bytes.Buffer, alloy *bytes.Buffer) bool {
	return sl.String() == alloy.String()
}
