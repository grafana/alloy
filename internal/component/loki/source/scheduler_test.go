package source

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"
)

var _ Source[int] = (*testSource)(nil)

func newTestSource(key int, exit bool) *testSource {
	return &testSource{
		key:        key,
		shouldExit: exit,
		running:    *atomic.NewBool(false),
	}
}

type testSource struct {
	key        int
	shouldExit bool
	numStarts  atomic.Int32
	running    atomic.Bool
}

func (t *testSource) Run(ctx, _ context.Context) {
	t.numStarts.Inc()
	t.running.Store(true)
	defer t.running.Store(false)

	if t.shouldExit {
		return
	}
	<-ctx.Done()
}

func (t *testSource) Key() int {
	return t.key
}

func (t *testSource) IsRunning() bool {
	return t.running.Load()
}

func TestScheduler(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	t.Run("should ignore duplicated keys", func(t *testing.T) {
		scheduler := NewScheduler[int]()
		scheduler.ScheduleSource(newTestSource(1, false))
		scheduler.ScheduleSource(newTestSource(1, false))
		require.Equal(t, 1, scheduler.Len())
		scheduler.Stop()
		require.Equal(t, 0, scheduler.Len())
	})

	t.Run("should stop running source", func(t *testing.T) {
		scheduler := NewScheduler[int]()
		source1, source2 := newTestSource(1, false), newTestSource(2, false)
		scheduler.ScheduleSource(source1)
		scheduler.ScheduleSource(source2)
		require.Equal(t, 2, scheduler.Len())

		scheduler.StopSource(source1)
		require.Eventually(t, func() bool {
			return !source1.IsRunning()
		}, 3*time.Second, 10*time.Millisecond)

		require.Equal(t, 1, scheduler.Len())
		require.True(t, scheduler.Contains(source2.Key()))

		scheduler.Stop()
		require.Equal(t, 0, scheduler.Len())
	})

	t.Run("source that stops", func(t *testing.T) {
		scheduler := NewScheduler[int]()
		s := newTestSource(1, true)
		scheduler.ScheduleSource(s)

		require.Eventually(t, func() bool {
			return !s.IsRunning()
		}, 3*time.Second, 10*time.Millisecond)

		scheduler.Stop()
	})
}

func TestScheduler_SourceWithRetry(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	scheduler := NewScheduler[int]()
	s := newTestSource(1, true)

	scheduler.ScheduleSource(NewSourceWithRetry(s, backoff.Config{
		MinBackoff: 20 * time.Millisecond,
		MaxBackoff: 10 * time.Second,
	}))

	time.Sleep(1 * time.Second)
	require.True(t, s.numStarts.Load() > 1)
	scheduler.Stop()
}

var _ Source[int] = (*contextSource)(nil)

// contextSource keeps the contexts it was run with so tests can inspect them.
type contextSource struct {
	key     int
	started chan struct{}
	ctx     context.Context
	stop    context.Context
}

func newContextSource(key int) *contextSource {
	return &contextSource{key: key, started: make(chan struct{})}
}

func (c *contextSource) Run(ctx, stop context.Context) {
	c.ctx, c.stop = ctx, stop
	close(c.started)
	<-ctx.Done()
}

func (c *contextSource) Key() int {
	return c.key
}

func TestScheduler_Contexts(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	t.Run("removing a source cancels ctx but not stop", func(t *testing.T) {
		scheduler := NewScheduler[int]()
		s := newContextSource(1)
		scheduler.ScheduleSource(s)
		<-s.started

		scheduler.StopSource(s)

		require.Eventually(t, func() bool {
			return s.ctx.Err() != nil
		}, 3*time.Second, 10*time.Millisecond)
		require.NoError(t, s.stop.Err())

		scheduler.Stop()
	})

	t.Run("stopping the scheduler cancels stop and ctx", func(t *testing.T) {
		scheduler := NewScheduler[int]()
		s := newContextSource(1)
		scheduler.ScheduleSource(s)
		<-s.started

		// Stop waits for all sources to return.
		scheduler.Stop()
		require.Error(t, s.stop.Err())
		require.Error(t, s.ctx.Err())
	})
}
