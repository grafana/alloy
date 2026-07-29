package logging

import (
	"bytes"
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
