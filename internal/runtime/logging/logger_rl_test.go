package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestLoggerRateLimitEndToEnd(t *testing.T) {
	defer goleak.VerifyNone(t) // PROOF: no goroutine leaked by this feature
	var buf bytes.Buffer
	// Tick is short (but long enough that the initial burst of 10 calls
	// lands in a single window even under -race) so the test can also
	// observe the dropped-count annotation, which slog-sampling only
	// attaches to the first admitted record of a *new* tick window (see
	// samber/slog-sampling middleware_threshold.go); it never appears
	// within the window that produced the drops.
	l, err := New(&buf, Options{
		Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: 100 * time.Millisecond, Threshold: 2, Rate: 0, MaxSignatures: 100},
	})
	require.NoError(t, err)
	log := l.Slog()
	for i := 0; i < 10; i++ {
		log.Info("floody")
	}
	require.Equal(t, 2, strings.Count(buf.String(), "floody")) // Threshold admitted within the window

	time.Sleep(300 * time.Millisecond) // let the tick window roll over
	log.Info("floody")
	require.Equal(t, 3, strings.Count(buf.String(), "floody"))
	require.Contains(t, buf.String(), "slog_sampling.dropped_count") // first admission of new window annotated with prior drops
}

func TestLoggerDistinctComponentsNoCrossSuppress(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 1, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)
	a := l.Slog().With("component_id", "a", "component_path", "/a")
	b := l.Slog().With("component_id", "b", "component_path", "/b")
	a.Info("same")
	a.Info("same") // 2nd dropped
	b.Info("same") // different component ⇒ own bucket ⇒ admitted
	require.Equal(t, 2, strings.Count(buf.String(), "msg=same"))
}

// TestLoggerSamePathDistinctComponentIDNoCrossSuppress guards against the
// bug where compMatcher keyed only on component_path (the parent/module
// path, e.g. "/" for every top-level component) and message/level. Two
// distinct top-level components sharing the same parent path but different
// component_id must not share a rate-limit bucket.
func TestLoggerSamePathDistinctComponentIDNoCrossSuppress(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 1, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)
	a := l.Slog().With("component_path", "/", "component_id", "comp.a")
	b := l.Slog().With("component_path", "/", "component_id", "comp.b")
	a.Info("same")
	a.Info("same") // 2nd dropped: same component, same signature
	b.Info("same") // different component_id, same path ⇒ own bucket ⇒ admitted
	require.Equal(t, 2, strings.Count(buf.String(), "msg=same"))
}

func TestLoggerDisabledByConfig(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt, RateLimiting: &RateLimitingOptions{Enabled: false}})
	require.NoError(t, err)
	log := l.Slog()
	for i := 0; i < 10; i++ {
		log.Info("noisy")
	}
	require.Equal(t, 10, strings.Count(buf.String(), "noisy"))
}

func TestUpdateInvalidRateLimitingLeavesStateUnchanged(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt, RateLimiting: &RateLimitingOptions{Enabled: false}})
	require.NoError(t, err)

	err = l.Update(Options{
		Level:        LevelError,
		Format:       FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: 0},
	})
	require.Error(t, err)

	// The level must not have been mutated: an Info record should still pass,
	// proving the invalid RateLimiting config was rejected before any other
	// state (level, format, writer) was applied.
	require.True(t, l.Enabled(context.Background(), slog.LevelInfo))
	l.Slog().Info("still-info")
	require.Contains(t, buf.String(), "still-info")
}

func TestLoggerLiveRetune(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 100, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)
	log := l.Slog() // logger captured BEFORE reload
	require.NoError(t, l.Update(Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 1, Rate: 0, MaxSignatures: 100}}))
	for i := 0; i < 5; i++ {
		log.Info("retuned")
	}
	require.Equal(t, 1, strings.Count(buf.String(), "retuned")) // new threshold applied live to pre-existing logger
}
