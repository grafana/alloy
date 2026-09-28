package source

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestLoopWaitsForOffset(t *testing.T) {
	var calls atomic.Int32
	l := startLoop(t.Context(), time.Hour, 200*time.Millisecond, nil, func(context.Context) { calls.Add(1) }, func() {})
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
	l := startLoop(t.Context(), interval, 0, nil, poll, func() { overruns.Add(1) })

	require.Eventually(t, func() bool { return calls.Load() >= 3 }, 5*time.Second, 10*time.Millisecond)
	l.stop()

	require.Equal(t, int32(1), maxRunning.Load(), "polls never overlap")
	require.GreaterOrEqual(t, overruns.Load(), int32(3))
	require.Less(t, time.Duration(maxGap.Load()), interval/2, "the next poll starts right after a slow poll")
}

func TestLoopStopWaits(t *testing.T) {
	var done atomic.Bool
	started := make(chan struct{})
	l := startLoop(t.Context(), time.Hour, 0, nil, func(ctx context.Context) {
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

// TestLoopNoPollAfterCancel checks that a tick and a cancel that are both
// ready do not start one more poll. The select picks at random, so the test
// repeats the race many times.
func TestLoopNoPollAfterCancel(t *testing.T) {
	interval := 5 * time.Millisecond
	for range 30 {
		ctx, cancel := context.WithCancel(t.Context())
		var calls atomic.Int32
		l := startLoop(ctx, interval, 0, nil, func(context.Context) {
			calls.Add(1)
			// Let a tick arrive, then cancel, so both are ready.
			time.Sleep(3 * interval)
			cancel()
		}, func() {})
		<-l.done
		require.Equal(t, int32(1), calls.Load(), "no poll may start after cancel")
		cancel()
	}
}

// TestLoopKickDuringOffset checks that a kick ends the offset wait early.
// The poll comes after a jitter below interval/10.
func TestLoopKickDuringOffset(t *testing.T) {
	kick := make(chan struct{}, 1)
	var calls atomic.Int32
	l := startLoop(t.Context(), time.Second, time.Hour, kick, func(context.Context) { calls.Add(1) }, func() {})
	defer l.stop()

	start := time.Now()
	kick <- struct{}{}
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, 5*time.Millisecond)
	require.Less(t, time.Since(start), 200*time.Millisecond, "the kicked poll waits at most interval/10")
}

// TestLoopKickDuringPoll checks that kicks during a poll give one more poll
// after it ends, and never two polls at the same time.
func TestLoopKickDuringPoll(t *testing.T) {
	kick := make(chan struct{}, 1)
	var calls, running, maxRunning atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	// The jitter stays below 200ms, far from the first tick at 2s.
	l := startLoop(t.Context(), 2*time.Second, 0, kick, func(context.Context) {
		if n := running.Inc(); n > maxRunning.Load() {
			maxRunning.Store(n)
		}
		if calls.Inc() == 1 {
			close(entered)
			<-release
		}
		running.Dec()
	}, func() {})
	defer l.stop()

	<-entered
	for range 3 {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
	close(release)

	require.Eventually(t, func() bool { return calls.Load() == 2 }, time.Second, 5*time.Millisecond)
	require.Never(t, func() bool { return calls.Load() > 2 }, 300*time.Millisecond, 10*time.Millisecond, "kicks coalesce into one poll")
	require.Equal(t, int32(1), maxRunning.Load(), "polls never overlap")
}

// TestLoopStopCancelsJitter checks that stop does not wait for, or run, a
// kicked poll whose jitter has not ended.
func TestLoopStopCancelsJitter(t *testing.T) {
	kick := make(chan struct{}, 1)
	var calls atomic.Int32
	// interval/10 is 6 minutes, so the jitter is still pending at stop.
	l := startLoop(t.Context(), time.Hour, time.Hour, kick, func(context.Context) { calls.Add(1) }, func() {})
	kick <- struct{}{}
	require.Eventually(t, func() bool { return len(kick) == 0 }, time.Second, 5*time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		l.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop waited for the jitter")
	}
	require.Zero(t, calls.Load())
}

func TestJitter(t *testing.T) {
	require.Zero(t, jitter(5*time.Nanosecond))
	for range 100 {
		j := jitter(time.Second)
		require.GreaterOrEqual(t, j, time.Duration(0))
		require.Less(t, j, 100*time.Millisecond)
	}
}

// TestLoopKickResetsTick checks that a kicked poll moves the next tick, so
// two polls do not come close together.
func TestLoopKickResetsTick(t *testing.T) {
	interval := 300 * time.Millisecond
	kick := make(chan struct{}, 1)
	var (
		mu    sync.Mutex
		polls []time.Time
	)
	l := startLoop(t.Context(), interval, 0, kick, func(context.Context) {
		mu.Lock()
		polls = append(polls, time.Now())
		mu.Unlock()
	}, func() {})
	defer l.stop()

	// Kick just before the first tick. The jitter is below 30ms.
	time.Sleep(250 * time.Millisecond)
	kick <- struct{}{}
	time.Sleep(2 * interval)
	l.stop()

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(polls), 3)
	for i := 1; i < len(polls); i++ {
		require.GreaterOrEqual(t, polls[i].Sub(polls[i-1]), interval/2, "poll %d came too soon after the one before", i)
	}
}
