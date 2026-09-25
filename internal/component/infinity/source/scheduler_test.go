package source

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestLoopWaitsForOffset(t *testing.T) {
	var calls atomic.Int32
	l := startLoop(t.Context(), time.Hour, 200*time.Millisecond, func(context.Context) { calls.Add(1) }, func() {})
	defer l.stop()

	time.Sleep(100 * time.Millisecond)
	require.Zero(t, calls.Load(), "the first poll waits for the offset")
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, 10*time.Millisecond)
}

func TestLoopNoOverlap(t *testing.T) {
	var (
		running, maxRunning, calls, overruns atomic.Int32
		lastEnd                              atomic.Int64
		maxGap                               atomic.Int64
	)
	interval := 200 * time.Millisecond
	poll := func(context.Context) {
		if end := lastEnd.Load(); end != 0 {
			gap := time.Now().UnixNano() - end
			if gap > maxGap.Load() {
				maxGap.Store(gap)
			}
		}
		n := running.Add(1)
		if n > maxRunning.Load() {
			maxRunning.Store(n)
		}
		time.Sleep(3 * interval)
		running.Add(-1)
		calls.Add(1)
		lastEnd.Store(time.Now().UnixNano())
	}
	l := startLoop(t.Context(), interval, 0, poll, func() { overruns.Add(1) })

	require.Eventually(t, func() bool { return calls.Load() >= 3 }, 5*time.Second, 10*time.Millisecond)
	l.stop()

	require.Equal(t, int32(1), maxRunning.Load(), "polls never overlap")
	require.GreaterOrEqual(t, overruns.Load(), int32(3))
	require.Less(t, time.Duration(maxGap.Load()), interval/2, "the next poll starts right after a slow poll")
}

func TestLoopStopWaits(t *testing.T) {
	var done atomic.Bool
	started := make(chan struct{})
	l := startLoop(t.Context(), time.Hour, 0, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		done.Store(true)
	}, func() {})
	<-started
	l.stop()
	require.True(t, done.Load(), "stop returns after the poll ends")
}

func TestRandomOffset(t *testing.T) {
	for range 100 {
		off := randomOffset(time.Second)
		require.GreaterOrEqual(t, off, time.Duration(0))
		require.Less(t, off, time.Second)
	}
}

func TestSelfMetricsDeleteQuery(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := newSelfMetrics(reg)
	require.NoError(t, err)

	m.pollFailures.WithLabelValues("a", reasonStatus).Inc()
	m.pollFailures.WithLabelValues("b", reasonStatus).Inc()
	m.deleteQuery("a")
	require.Equal(t, 1, testutil.CollectAndCount(m.pollFailures))
}
