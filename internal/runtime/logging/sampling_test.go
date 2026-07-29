package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
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
