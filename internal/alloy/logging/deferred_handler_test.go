package logging

import (
	"bytes"
	"context"
	"github.com/go-kit/log/level"
	"github.com/stretchr/testify/require"
	"log/slog"
	"strings"
	"testing"
)

func TestDeferredHandlerWritingToBothLoggers(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := NewDeferred(bb)
	slogger := slog.New(l.deferredSlog)
	require.NoError(t, err)
	l.Log("msg", "this should happen before")
	slogger.Log(context.Background(), slog.LevelInfo, "this should happen after)")

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
	logInfo(sl, alloy, "test")
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
	logInfo(sl, alloy, "test")
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
	logError(sl, alloy, "test")
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
				logInfo(sl, alloy, "test")
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
				logInfo(sl, alloy, "test")
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
				logInfo(sl, alloy, "test")
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
				logInfo(sl, alloy, "test")
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

func logInfo(sl *slog.Logger, alloyL *slog.Logger, msg string) {
	ctx := context.Background()
	sl.Log(ctx, slog.LevelInfo, msg)
	alloyL.Log(ctx, slog.LevelInfo, msg)
}

func logError(sl *slog.Logger, alloyL *slog.Logger, msg string) {
	ctx := context.Background()
	sl.Log(ctx, slog.LevelError, msg)
	alloyL.Log(ctx, slog.LevelError, msg)
}

func equal(sl *bytes.Buffer, alloy *bytes.Buffer) bool {
	return sl.String() == alloy.String()
}
