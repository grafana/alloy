package logging

import (
	"bytes"
	"context"
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
// terminal handler, keys via our Matcher, and admits `Threshold` per tick then
// drops (rate=0). This validates the exact library wiring the design relies on.
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

func TestCompMatcherKeysOnPathLevelMessage(t *testing.T) {
	mk := func(path string, level slog.Level, msg string) string {
		ctx := withComponent(context.Background(), componentInfo{path: path})
		r := slog.NewRecord(time.Time{}, level, msg, 0)
		return compMatcher(ctx, &r)
	}

	base := mk("/a", slog.LevelInfo, "hello")

	require.Equal(t, base, mk("/a", slog.LevelInfo, "hello"), "identical path/level/message should produce the same key")
	require.NotEqual(t, base, mk("/b", slog.LevelInfo, "hello"), "different path should produce a different key")
	require.NotEqual(t, base, mk("/a", slog.LevelWarn, "hello"), "different level should produce a different key")
	require.NotEqual(t, base, mk("/a", slog.LevelInfo, "goodbye"), "different message should produce a different key")
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
	// A root that drops everything, to prove empty-message goes to `bare` (term) not root.
	// AbsoluteSamplingOption panics when Max == 0, so we build the drop-all root
	// with ThresholdSamplingOption{Threshold: 0, Rate: 0} instead, which admits
	// nothing (confirmed against slog-sampling v1.6.0 in Task 2's spike).
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
