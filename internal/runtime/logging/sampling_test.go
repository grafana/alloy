package logging

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	slogsampling "github.com/samber/slog-sampling"
	"github.com/samber/slog-sampling/buffer"
	"github.com/stretchr/testify/require"
)

// TestSpikeThresholdAdmitsThenDrops confirms the Threshold middleware wraps a
// terminal handler, keys records with our Matcher, and admits `Threshold`
// records per tick, then drops the rest (rate=0).
func TestSpikeThresholdAdmitsThenDrops(t *testing.T) {
	var buf bytes.Buffer
	terminal := slog.NewTextHandler(&buf, nil)

	opt := slogsampling.ThresholdSamplingOption{
		Tick:      time.Hour, // one window for the whole test
		Threshold: 3,
		Rate:      0,
		Matcher:   func(ctx context.Context, r *slog.Record) string { return r.Message },
		Buffer:    buffer.NewLRUBuffer[string](100),
	}
	h := opt.NewMiddleware()(terminal)
	logger := slog.New(h)
	for i := 0; i < 10; i++ {
		logger.Info("spam")
	}
	require.Equal(t, 3, strings.Count(buf.String(), "spam"), "expected exactly Threshold admitted")
}

func TestSniffComponent(t *testing.T) {
	t.Run("component_id wins over controller_id", func(t *testing.T) {
		c := sniffComponent(componentInfo{}, []slog.Attr{
			slog.String("controller_id", "ctrl-1"),
			slog.String("component_id", "comp-1"),
		})
		require.Equal(t, "comp-1", c.id)
	})

	t.Run("controller_id used when component_id absent", func(t *testing.T) {
		c := sniffComponent(componentInfo{}, []slog.Attr{
			slog.String("controller_id", "ctrl-1"),
		})
		require.Equal(t, "ctrl-1", c.id)
	})

	t.Run("component_id already set on base is not overridden by controller_id", func(t *testing.T) {
		c := sniffComponent(componentInfo{id: "existing"}, []slog.Attr{
			slog.String("controller_id", "ctrl-1"),
		})
		require.Equal(t, "existing", c.id)
	})

	t.Run("component_path captured", func(t *testing.T) {
		c := sniffComponent(componentInfo{}, []slog.Attr{
			slog.String("component_path", "/foo/bar"),
		})
		require.Equal(t, "/foo/bar", c.path)
	})
}

// TestSniffComponentControllerPath checks a past bug, where sniffComponent
// ignored controller_path. Controller log lines carry controller_id and
// controller_path, not component_id and component_path. Without this case,
// every controller log line got path="", so two nested controllers with the
// same leaf controller_id under different parents shared one rate-limit
// bucket and cross-suppressed each other.
func TestSniffComponentControllerPath(t *testing.T) {
	t.Run("controller_path used when component_path absent", func(t *testing.T) {
		c := sniffComponent(componentInfo{}, []slog.Attr{
			slog.String("controller_id", "controller_id"),
			slog.String("controller_path", "controller_path"),
		})
		require.Equal(t, componentInfo{id: "controller_id", path: "controller_path"}, c)
	})

	t.Run("component_path wins over controller_path", func(t *testing.T) {
		c := sniffComponent(componentInfo{}, []slog.Attr{
			slog.String("controller_path", "/ctrl"),
			slog.String("component_path", "/comp"),
		})
		require.Equal(t, "/comp", c.path)
	})

	t.Run("component_path wins over controller_path regardless of attr order", func(t *testing.T) {
		c := sniffComponent(componentInfo{}, []slog.Attr{
			slog.String("component_path", "/comp"),
			slog.String("controller_path", "/ctrl"),
		})
		require.Equal(t, "/comp", c.path)
	})
}

func TestCompMatcherKeysOnPathIDLevelMessage(t *testing.T) {
	mk := func(path, id string, level slog.Level, msg string) string {
		ctx := withComponent(context.Background(), componentInfo{path: path, id: id})
		r := slog.NewRecord(time.Time{}, level, msg, 0)
		return compMatcher(ctx, &r)
	}

	base := mk("/a", "comp.a", slog.LevelInfo, "hello")

	require.Equal(t, base, mk("/a", "comp.a", slog.LevelInfo, "hello"), "identical path/id/level/message should produce the same key")
	require.NotEqual(t, base, mk("/b", "comp.a", slog.LevelInfo, "hello"), "different path should produce a different key")
	require.NotEqual(t, base, mk("/a", "comp.b", slog.LevelInfo, "hello"), "different component id (same path) should produce a different key")
	require.NotEqual(t, base, mk("/a", "comp.a", slog.LevelWarn, "hello"), "different level should produce a different key")
	require.NotEqual(t, base, mk("/a", "comp.a", slog.LevelInfo, "goodbye"), "different message should produce a different key")
}

func TestBuildRootDisabledReturnsTerminal(t *testing.T) {
	terminal := slog.NewTextHandler(&bytes.Buffer{}, nil)
	got := buildRoot(RateLimitingOptions{Enabled: false}, terminal, nil)
	require.Same(t, terminal, got)
}

func newTestInjector(t *testing.T, root slog.Handler, bare slog.Handler) *samplingInjector {
	t.Helper()
	var holder atomic.Pointer[versionedHandler]
	holder.Store(&versionedHandler{version: 1, h: root})
	return newSamplingInjector(&holder, bare)
}

func TestInjectorRendersAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	term := slog.NewTextHandler(&buf, nil)
	inj := newTestInjector(t, term, term) // no sampling, just render
	h := inj.WithAttrs([]slog.Attr{slog.String("component_id", "x")}).WithGroup("g").WithAttrs([]slog.Attr{slog.String("k", "v")}).(*samplingInjector)
	require.Equal(t, "x", h.comp.id)
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "hi", 0)
	require.NoError(t, h.Handle(context.Background(), rec))
	out := buf.String()
	require.Contains(t, out, "component_id=x")
	require.Contains(t, out, "g.k=v") // group-nested attr rendered natively
}

func TestInjectorEmptyMessageBypassesSampler(t *testing.T) {
	var termBuf bytes.Buffer
	term := slog.NewTextHandler(&termBuf, nil)
	// A root that drops everything, to prove empty-message records go to
	// `bare` (term), not root. AbsoluteSamplingOption panics when Max == 0,
	// so we build the drop-all root with ThresholdSamplingOption{Threshold:
	// 0, Rate: 0} instead, which admits nothing.
	dropAll := slogsampling.ThresholdSamplingOption{
		Tick:      time.Hour,
		Threshold: 0,
		Rate:      0,
		Matcher:   func(ctx context.Context, r *slog.Record) string { return "" },
		Buffer:    buffer.NewLRUBuffer[string](10),
	}.NewMiddleware()(slog.NewTextHandler(&bytes.Buffer{}, nil))
	inj := newTestInjector(t, dropAll, term)
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "", 0)
	rec.AddAttrs(slog.String("k", "v"))
	require.NoError(t, inj.Handle(context.Background(), rec))
	require.Contains(t, termBuf.String(), "k=v") // empty-msg reached bare terminal
}

// TestInjectorEnabledMatchesTerminalLevel checks that Enabled calls the bare
// terminal handler, whose leveler reflects the current, live-updatable
// level, not the sampling-wrapped root. Root reports everything enabled
// (LevelDebug), while bare is gated at Info. If Enabled called root instead
// of bare, the LevelDebug assertion below would fail.
func TestInjectorEnabledMatchesTerminalLevel(t *testing.T) {
	root := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	bare := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	inj := newTestInjector(t, root, bare)

	require.False(t, inj.Enabled(context.Background(), slog.LevelDebug), "Enabled must reflect the terminal's level, not the sampling root's")
	require.True(t, inj.Enabled(context.Background(), slog.LevelInfo))
}

func TestInjectorReDerivesOnVersionBump(t *testing.T) {
	var bufA, bufB bytes.Buffer
	termA := slog.NewTextHandler(&bufA, nil)
	termB := slog.NewTextHandler(&bufB, nil)
	var holder atomic.Pointer[versionedHandler]
	holder.Store(&versionedHandler{version: 1, h: termA})
	inj := newSamplingInjector(&holder, termA)
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "m", 0)
	require.NoError(t, inj.Handle(context.Background(), rec))
	require.Contains(t, bufA.String(), "m")
	holder.Store(&versionedHandler{version: 2, h: termB}) // reload
	require.NoError(t, inj.Handle(context.Background(), rec))
	require.Contains(t, bufB.String(), "m") // now routed to the new root
}
