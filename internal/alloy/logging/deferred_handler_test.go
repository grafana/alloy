package logging

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

func TestSlogHandle(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := NewDeferred(bb)
	require.NoError(t, err)

	bbSl := &bytes.Buffer{}
	sl := slog.NewJSONHandler(bbSl, &slog.HandlerOptions{
		AddSource:   true,
		Level:       nil,
		ReplaceAttr: replace,
	})
	handle(sl, l.DeferredSlog, slog.Record{
		Message: "test",
	})
	err = l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, bb.String() == bbSl.String())
}

func TestSlogHandleWithAttr(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := NewDeferred(bb)
	require.NoError(t, err)

	bbSl := &bytes.Buffer{}
	sl := slog.NewJSONHandler(bbSl, &slog.HandlerOptions{
		AddSource:   true,
		Level:       nil,
		ReplaceAttr: replace,
	})

	slogH, df := withAttrs(sl, l.DeferredSlog, []slog.Attr{
		{
			"attr1",
			slog.StringValue("value1"),
		},
	})
	handle(slogH, df, slog.Record{
		Message: "test",
	})
	err = l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, bb.String() == bbSl.String())
}

func TestSlogHandleWithAttrNested(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := NewDeferred(bb)
	require.NoError(t, err)

	bbSl := &bytes.Buffer{}
	sl := slog.NewJSONHandler(bbSl, &slog.HandlerOptions{
		AddSource:   true,
		Level:       nil,
		ReplaceAttr: replace,
	})

	slogH, df := withAttrs(sl, l.DeferredSlog, []slog.Attr{
		{
			"attr1",
			slog.StringValue("value1"),
		},
	})

	slogH, df = withAttrs(slogH, df, []slog.Attr{
		{
			"nestedattr1",
			slog.StringValue("nestedvalue1"),
		},
	})
	handle(slogH, df, slog.Record{
		Message: "test",
	})
	err = l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})
	require.NoError(t, err)
	require.True(t, bb.String() == bbSl.String())
}

func TestSlogHandleWithGroup(t *testing.T) {
	bb := &bytes.Buffer{}
	l, err := NewDeferred(bb)
	require.NoError(t, err)

	bbSl := &bytes.Buffer{}
	sl := slog.NewJSONHandler(bbSl, &slog.HandlerOptions{
		AddSource: true,
		Level:     nil,
	})
	slogH, df := withGroup(sl, l.DeferredSlog, "gr1")
	slogH, df = withAttrs(sl, l.DeferredSlog, []slog.Attr{
		{
			"nestedattr1",
			slog.StringValue("nestedvalue1"),
		},
	})

	handle(slogH, df, slog.Record{
		Message: "test",
	})
	err = l.Update(Options{
		Level:   "debug",
		Format:  "json",
		WriteTo: nil,
	})

	require.NoError(t, err)
	require.True(t, bb.String() == bbSl.String())
}

func TestPlain(t *testing.T) {
	bbSl := &bytes.Buffer{}

	sl := slog.NewJSONHandler(bbSl, &slog.HandlerOptions{
		AddSource: true,
		Level:     nil,
	})
	l := slog.New(sl)
	l = l.WithGroup("grp")
	l.LogAttrs(context.Background(), slog.LevelInfo, "test", []slog.Attr{
		slog.String("name", "bob"),
	}...)
	l.Log(context.Background(), slog.LevelInfo, "blah")
	println(bbSl.String())
}

func withAttrs(sl slog.Handler, alloyL slog.Handler, attrs []slog.Attr) (slog.Handler, slog.Handler) {
	return sl.WithAttrs(attrs), alloyL.WithAttrs(attrs)
}

func withGroup(sl *slog.Logger, alloyL *slog.Logger, group string) (*slog.Logger, *slog.Logger) {
	return sl.WithGroup(group), alloyL.DeferredSlog
}

func handle(sl slog.Handler, alloyL slog.Handler, r slog.Record) {
	ctx := context.Background()
	sl.Handle(ctx, r)
	alloyL.Handle(ctx, r)
}
