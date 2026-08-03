package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestLoggerRateLimitEndToEnd(t *testing.T) {
	defer goleak.VerifyNone(t) // PROOF: no goroutine leaked by this feature
	var buf bytes.Buffer
	// Tick is short, but long enough that the first burst of 10 calls lands
	// in one window, even under -race. This lets the test also see the
	// dropped-count annotation. slog-sampling adds this annotation only to
	// the first admitted record of a new tick window, never to the window
	// that produced the drops.
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

// TestLoggerSamePathDistinctComponentIDNoCrossSuppress checks a past bug,
// where compMatcher keyed only on component_path (the parent path, for
// example "/" for every top-level component), message, and level. Two
// top-level components with the same parent path but different
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

// ctxVariantKey is an unexported context key used solely to build a non-
// context.Background() ctx for TestInjectorComponentKeyingViaContextVariant.
type ctxVariantKey struct{}

// TestInjectorComponentKeyingViaContextVariant checks Handle's fallback
// path: when Handle gets a ctx other than context.Background(), for example
// from slog's *Context logging methods, it must still add component
// identity through withComponent(ctx, s.comp), not the cached bgCtx. This
// keeps distinct components with the same signature from suppressing each
// other.
func TestInjectorComponentKeyingViaContextVariant(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 1, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)
	a := l.Slog().With("component_path", "/", "component_id", "comp.a")
	b := l.Slog().With("component_path", "/", "component_id", "comp.b")

	// Deliberately not context.Background(), to exercise Handle's
	// withComponent(ctx, s.comp) fallback rather than the cached bgCtx.
	ctx := context.WithValue(context.Background(), ctxVariantKey{}, 1)
	require.NotEqual(t, context.Background(), ctx)

	a.Log(ctx, slog.LevelInfo, "same")
	a.Log(ctx, slog.LevelInfo, "same") // 2nd dropped: same component, same signature
	b.Log(ctx, slog.LevelInfo, "same") // different component_id ⇒ own bucket ⇒ admitted despite the shared non-Background ctx
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

// TestUpdateSameOptionsPreservesBudget checks that re-applying the same
// RateLimitingOptions on every Update, for example from an unrelated config
// reload, does not reset the sampler's per-signature counters. Before this
// fix, Update always rebuilt the sampler with a fresh LRU and counters on
// every call, so the repeated call below would wrongly re-admit the
// already-throttled line.
func TestUpdateSameOptionsPreservesBudget(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	opts := Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 2, Rate: 0, MaxSignatures: 100}}
	l, err := New(&buf, opts)
	require.NoError(t, err)
	log := l.Slog()

	log.Info("steady")
	log.Info("steady")
	log.Info("steady") // 3rd: over threshold, dropped
	require.Equal(t, 2, strings.Count(buf.String(), "msg=steady"))

	// Re-apply the identical options (simulating an unrelated config
	// reload triggering LoggingConfigNode.Evaluate -> Update again).
	require.NoError(t, l.Update(opts))

	log.Info("steady") // still over the ORIGINAL window's budget: must stay dropped
	require.Equal(t, 2, strings.Count(buf.String(), "msg=steady"))
}

// TestUpdateChangedOptionsRebuilds confirms that when rate_limiting options
// change across an Update call, the new configuration takes effect for an
// existing logger. A change must still trigger a rebuild, not just skip it
// like unchanged options do.
func TestUpdateChangedOptionsRebuilds(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 2, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)
	log := l.Slog()

	require.NoError(t, l.Update(Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 1, Rate: 0, MaxSignatures: 100}}))

	log.Info("changed")
	log.Info("changed") // 2nd: over the NEW threshold of 1, dropped
	require.Equal(t, 1, strings.Count(buf.String(), "msg=changed"))
}

// TestMetricInitAfterUpdateStillCounts checks that InitRateLimitMetrics
// takes effect even when it runs after the first Update already built the
// root handler. Before the fix, buildRoot closed over the *rateLimitMetrics
// value at build time; a nil value at that point (metrics not yet
// initialized) meant the counter stayed silently disabled forever, no
// matter when InitRateLimitMetrics ran afterward.
func TestMetricInitAfterUpdateStillCounts(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	// New runs the first Update with no metrics registered yet.
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 1, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)

	// Metrics are initialized only now, after the root handler already exists.
	reg := prometheus.NewRegistry()
	l.InitRateLimitMetrics(reg)

	log := l.Slog()
	log.Info("late-metric")
	log.Info("late-metric") // 2nd dropped: over threshold

	mfs, err := reg.Gather()
	require.NoError(t, err)
	var total float64
	for _, mf := range mfs {
		if mf.GetName() != "alloy_logging_suppressed_lines_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			total += m.GetCounter().GetValue()
		}
	}
	require.GreaterOrEqual(t, total, float64(1), "suppressed-lines counter must increment even when InitRateLimitMetrics runs after the first Update")
}

// TestConcurrentUpdatesNoRace calls Update from many goroutines at once,
// with valid options that may differ. It checks that -race stays clean,
// nothing panics, and the logger still works afterward.
func TestConcurrentUpdatesNoRace(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: LevelInfo, Format: FormatLogfmt,
		RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: 10, Rate: 0, MaxSignatures: 100}})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				threshold := uint64(1 + (g+i)%5)
				err := l.Update(Options{Level: LevelInfo, Format: FormatLogfmt,
					RateLimiting: &RateLimitingOptions{Enabled: true, Tick: time.Hour, Threshold: threshold, Rate: 0, MaxSignatures: 100}})
				require.NoError(t, err)
			}
		}(g)
	}
	wg.Wait()

	l.Slog().Info("post-concurrent-update")
	require.Contains(t, buf.String(), "post-concurrent-update")
}
