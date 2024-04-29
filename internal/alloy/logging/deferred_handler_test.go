package logging

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

// All these tests build a normal slog handler, and a deferred handler then run the same operations on both and compare.

func TestSlogHandle(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)
	log(sl, alloy, "test")
	err := l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, equal(bb, bbSl))
}

func TestSlogHandleWithAttr(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)

	sl, alloy = withAttrs(sl, alloy, "attr1", "value1")
	log(sl, alloy, "test")
	err := l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, equal(bb, bbSl))
}

func TestSlogHandleWithAttrNested(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)

	sl, alloy = withAttrs(sl, alloy, "attr1", "value1")
	sl, alloy = withAttrs(sl, alloy, "nestedattr1", "nestedvalue1")
	log(sl, alloy, "test")
	err := l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, equal(bb, bbSl))
}

func TestSlogHandleWithGroup(t *testing.T) {
	bb := &bytes.Buffer{}
	bbSl := &bytes.Buffer{}
	sl, alloy, l := newLoggers(t, bb, bbSl)
	sl, alloy = withGroup(sl, alloy, "gr1")
	sl, alloy = withAttrs(sl, alloy, "nestedattr1", "nestedvalue1")
	log(sl, alloy, "test")
	err := l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})

	require.NoError(t, err)
	require.True(t, equal(bb, bbSl))
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
	alloy := slog.New(l.DeferredSlog)
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

func handle(sl slog.Handler, alloyL slog.Handler, r slog.Record) {
	ctx := context.Background()
	sl.Handle(ctx, r)
	alloyL.Handle(ctx, r)
}

func log(sl *slog.Logger, alloyL *slog.Logger, msg string) {
	ctx := context.Background()
	sl.Log(ctx, slog.LevelInfo, msg)
	alloyL.Log(ctx, slog.LevelInfo, msg)
}

func equal(sl *bytes.Buffer, alloy *bytes.Buffer) bool {
	return sl.String() == alloy.String()
}
