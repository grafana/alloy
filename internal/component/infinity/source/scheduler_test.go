package source

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// The loop tests use synctest, so the clock is virtual and the timing
// checks are exact.

func TestLoopWaitsForOffset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		offset := 200 * time.Millisecond
		var calls atomic.Int32
		l := startLoop(t.Context(), time.Hour, offset, nil, func(context.Context) { calls.Add(1) }, func() {})
		defer l.stop()

		time.Sleep(offset - time.Nanosecond)
		synctest.Wait()
		require.Zero(t, calls.Load(), "the first poll waits for the offset")
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Equal(t, int32(1), calls.Load())
	})
}

// TestLoopNoOverlap checks that a slow poll delays the next one. The next
// poll starts when the slow poll ends, because the ticker keeps one tick.
func TestLoopNoOverlap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		interval := 200 * time.Millisecond
		var (
			running, maxRunning, overruns atomic.Int32
			mu                            sync.Mutex
			starts                        []time.Duration
		)
		begin := time.Now()
		poll := func(context.Context) {
			mu.Lock()
			starts = append(starts, time.Since(begin))
			mu.Unlock()
			if n := running.Inc(); n > maxRunning.Load() {
				maxRunning.Store(n)
			}
			time.Sleep(3 * interval)
			running.Dec()
		}
		l := startLoop(t.Context(), interval, 0, nil, poll, func() { overruns.Inc() })

		// Stop during the third poll. stop waits for it to end.
		time.Sleep(8 * interval)
		l.stop()

		require.Equal(t, int32(1), maxRunning.Load(), "polls never overlap")
		require.Equal(t, int32(3), overruns.Load())
		require.Equal(t, []time.Duration{0, 3 * interval, 6 * interval}, starts)
	})
}

// TestLoopOverrun checks the limit of the overrun callback. A poll that
// takes exactly one interval is not an overrun.
func TestLoopOverrun(t *testing.T) {
	interval := time.Second
	tests := map[time.Duration]int32{
		interval:                   0,
		interval + time.Nanosecond: 1,
	}
	for d, want := range tests {
		t.Run(d.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls, overruns atomic.Int32
				l := startLoop(t.Context(), interval, 0, nil, func(context.Context) {
					if calls.Inc() == 1 {
						time.Sleep(d)
					}
				}, func() { overruns.Inc() })

				time.Sleep(3 * interval / 2)
				l.stop()
				require.Equal(t, int32(2), calls.Load())
				require.Equal(t, want, overruns.Load())
			})
		})
	}
}

func TestLoopStopWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var done atomic.Bool
		started := make(chan struct{})
		l := startLoop(t.Context(), time.Hour, 0, nil, func(ctx context.Context) {
			close(started)
			<-ctx.Done()
			time.Sleep(time.Second)
			done.Store(true)
		}, func() {})
		<-started
		l.stop()
		require.True(t, done.Load(), "stop returns after the poll ends")
	})
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
	m := newSelfMetrics(reg)

	m.pollFailures.WithLabelValues("a", reasonStatus).Inc()
	m.pollFailures.WithLabelValues("b", reasonStatus).Inc()
	m.deleteQuery("a")
	require.Equal(t, 1, testutil.CollectAndCount(m.pollFailures))
}

// TestLoopNoPollAfterCancel checks that a tick and a cancel that are both
// ready do not start one more poll. The select picks at random, so the test
// repeats the race many times.
func TestLoopNoPollAfterCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		interval := time.Second
		for range 100 {
			ctx, cancel := context.WithCancel(t.Context())
			var calls atomic.Int32
			l := startLoop(ctx, interval, 0, nil, func(context.Context) {
				calls.Inc()
				// Let a tick arrive, then cancel, so both are ready.
				time.Sleep(3 * interval)
				cancel()
			}, func() {})
			<-l.done
			require.Equal(t, int32(1), calls.Load(), "no poll may start after cancel")
		}
	})
}

// TestLoopKickDuringOffset checks that a kick ends the offset wait early.
// The poll comes after a jitter below interval/10.
func TestLoopKickDuringOffset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		interval := time.Second
		kick := make(chan struct{}, 1)
		var calls atomic.Int32
		l := startLoop(t.Context(), interval, time.Hour, kick, func(context.Context) { calls.Inc() }, func() {})
		defer l.stop()

		synctest.Wait()
		require.Zero(t, calls.Load())
		kick <- struct{}{}
		time.Sleep(interval / 10)
		synctest.Wait()
		require.Equal(t, int32(1), calls.Load(), "the kicked poll waits less than interval/10")
	})
}

// TestLoopKickDuringPoll checks that kicks during a poll give one more poll
// after it ends, and never two polls at the same time.
func TestLoopKickDuringPoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		interval := 2 * time.Second
		kick := make(chan struct{}, 1)
		var calls, running, maxRunning atomic.Int32
		entered := make(chan struct{})
		release := make(chan struct{})
		l := startLoop(t.Context(), interval, 0, kick, func(context.Context) {
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

		time.Sleep(interval / 10)
		synctest.Wait()
		require.Equal(t, int32(2), calls.Load())
		// The kicked poll moves the next tick to one interval after it.
		time.Sleep(interval - interval/10 - time.Nanosecond)
		synctest.Wait()
		require.Equal(t, int32(2), calls.Load(), "kicks coalesce into one poll")
		require.Equal(t, int32(1), maxRunning.Load(), "polls never overlap")
	})
}

// TestLoopPollServesWaitingKick checks that a poll serves a kick that waits
// in the channel. When the first poll wins the select over the kick, the
// kick must not cause a second poll. select picks at random, so the test
// runs many loops.
func TestLoopPollServesWaitingKick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const loops = 50
		interval := 2 * time.Second
		calls := make([]atomic.Int32, loops)
		for i := range loops {
			kick := make(chan struct{}, 1)
			kick <- struct{}{}
			l := startLoop(t.Context(), interval, 0, kick, func(context.Context) { calls[i].Inc() }, func() {})
			defer l.stop()
		}

		// A second poll from the kick would come within interval/10. The
		// next tick comes one interval after the first poll.
		time.Sleep(interval - time.Nanosecond)
		synctest.Wait()
		for i := range loops {
			require.Equal(t, int32(1), calls[i].Load(), "loop %d", i)
		}
	})
}

// TestLoopStopCancelsJitter checks that stop does not wait for, or run, a
// kicked poll whose jitter has not ended.
func TestLoopStopCancelsJitter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		kick := make(chan struct{}, 1)
		var calls atomic.Int32
		// interval/10 is 6 minutes, so the jitter is still pending at stop.
		l := startLoop(t.Context(), time.Hour, time.Hour, kick, func(context.Context) { calls.Inc() }, func() {})
		kick <- struct{}{}
		synctest.Wait()
		require.Empty(t, kick)

		start := time.Now()
		l.stop()
		require.Equal(t, start, time.Now(), "stop waited for the jitter")
		require.Zero(t, calls.Load())
	})
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
	synctest.Test(t, func(t *testing.T) {
		interval := 300 * time.Millisecond
		kick := make(chan struct{}, 1)
		var (
			mu    sync.Mutex
			polls []time.Duration
		)
		begin := time.Now()
		l := startLoop(t.Context(), interval, 0, kick, func(context.Context) {
			mu.Lock()
			polls = append(polls, time.Since(begin))
			mu.Unlock()
		}, func() {})

		// Kick just before the first tick.
		kickAt := 250 * time.Millisecond
		time.Sleep(kickAt)
		kick <- struct{}{}
		time.Sleep(3*interval - kickAt)
		l.stop()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, polls, 4)
		require.Zero(t, polls[0])
		require.GreaterOrEqual(t, polls[1], kickAt)
		require.Less(t, polls[1], kickAt+interval/10)
		require.Equal(t, interval, polls[2]-polls[1], "the kicked poll resets the tick")
		require.Equal(t, interval, polls[3]-polls[2])
	})
}
