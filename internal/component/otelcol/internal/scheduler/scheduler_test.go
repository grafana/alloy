package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
)

func TestScheduler(t *testing.T) {
	t.Run("Scheduled components get started", func(t *testing.T) {
		var (
			l  = util.TestLogger(t)
			cs = scheduler.New(l)
			h  = scheduler.NewHost(l)
		)

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(componenttest.TestContext(t))
			require.NoError(t, err)
		}()

		// Schedule our component, which should notify the started trigger once it is
		// running.
		component, started, _ := newTriggerComponent()
		cs.Schedule(h, component)
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
	})

	t.Run("Unscheduled components get stopped", func(t *testing.T) {
		var (
			l  = util.TestLogger(t)
			cs = scheduler.New(l)
			h  = scheduler.NewHost(l)
		)

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(componenttest.TestContext(t))
			require.NoError(t, err)
		}()

		// Schedule our component, which should notify the started and stopped
		// trigger once it starts and stops respectively.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(h, component)

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(h)
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
	})

	t.Run("Pause callbacks are called", func(t *testing.T) {
		var (
			pauseCalls  = &atomic.Int32{}
			resumeCalls = &atomic.Int32{}
			l           = util.TestLogger(t)
			cs          = scheduler.NewWithPauseCallbacks(
				l,
				func() { pauseCalls.Inc() },
				func() { resumeCalls.Inc() },
			)
			h = scheduler.NewHost(l)
		)
		ctx, cancel := context.WithCancel(context.Background())

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(ctx)
			require.NoError(t, err)
		}()

		// Schedule our component, which should notify the started and stopped
		// trigger once it starts and stops respectively.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(h, component)

		toInt := func(a *atomic.Int32) int { return int(a.Load()) }

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 0, toInt(pauseCalls), "pause callbacks should not be called on first run")
			assert.Equal(t, 1, toInt(resumeCalls), "resume callback should be called on first run")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(h)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 1, toInt(pauseCalls), "pause callback should be called on second run")
			assert.Equal(t, 2, toInt(resumeCalls), "resume callback should be called on second run")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")

		// Stop the scheduler
		cancel()

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 2, toInt(pauseCalls), "pause callback should be called on shutdown")
			assert.Equal(t, 3, toInt(resumeCalls), "resume callback should be called on shutdown")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")
	})

	t.Run("Running components get stopped on shutdown", func(t *testing.T) {
		var (
			l  = util.TestLogger(t)
			cs = scheduler.New(l)
			h  = scheduler.NewHost(l)
		)

		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(ctx)
			require.NoError(t, err)
		}()

		// Schedule our component which will notify our trigger when Shutdown gets
		// called.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(h, component)

		// Wait for the component to start, and then stop our scheduler, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cancel()
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
	})
}

func newTriggerComponent() (component otelcomponent.Component, started, stopped *util.WaitTrigger) {
	started = util.NewWaitTrigger()
	stopped = util.NewWaitTrigger()

	component = &fakeComponent{
		StartFunc: func(_ context.Context, _ otelcomponent.Host) error {
			started.Trigger()
			return nil
		},
		ShutdownFunc: func(_ context.Context) error {
			stopped.Trigger()
			return nil
		},
	}

	return
}

type fakeComponent struct {
	StartFunc    func(ctx context.Context, host otelcomponent.Host) error
	ShutdownFunc func(ctx context.Context) error
}

var _ otelcomponent.Component = (*fakeComponent)(nil)

func (fc *fakeComponent) Start(ctx context.Context, host otelcomponent.Host) error {
	if fc.StartFunc != nil {
		fc.StartFunc(ctx, host)
	}
	return nil
}

func (fc *fakeComponent) Shutdown(ctx context.Context) error {
	if fc.ShutdownFunc != nil {
		return fc.ShutdownFunc(ctx)
	}
	return nil
}
