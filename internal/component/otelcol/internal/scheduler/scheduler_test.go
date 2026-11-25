package scheduler_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"

	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
)

func TestScheduler(t *testing.T) {
	t.Run("Scheduled components get started", func(t *testing.T) {
		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.New(l.Slog())
			h  = scheduler.NewHost()
		)

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(componenttest.TestContext(t))
			require.NoError(t, err)
		}()

		// Schedule our component, which should notify the started trigger once it is
		// running.
		component, started, _ := newTriggerComponent()
		cs.Schedule(t.Context(), func() {}, h, component)
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
	})

	t.Run("Unscheduled components get stopped", func(t *testing.T) {
		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.New(l.Slog())
			h  = scheduler.NewHost()
		)

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(componenttest.TestContext(t))
			require.NoError(t, err)
		}()

		// Schedule our component, which should notify the started and stopped
		// trigger once it starts and stops respectively.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(t.Context(), func() {}, h, component)

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(t.Context(), func() {}, h)
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
	})

	t.Run("Pause callbacks are called", func(t *testing.T) {
		var (
			pauseCalls  = &atomic.Int32{}
			resumeCalls = &atomic.Int32{}
			l           = util.TestAlloyLogger(t)
			cs          = scheduler.NewWithPauseCallbacks(
				l.Slog(),
				func() { pauseCalls.Inc() },
				func() { resumeCalls.Inc() },
			)
			h = scheduler.NewHost()
		)
		ctx, cancel := context.WithCancel(t.Context())

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(ctx)
			require.NoError(t, err)
		}()

		toInt := func(a *atomic.Int32) int { return int(a.Load()) }

		// Schedule our component, which should notify the started and stopped
		// trigger once it starts and stops respectively.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(ctx, func() {}, h, component)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 1, toInt(pauseCalls), "pause callbacks should be called on schedule")
			assert.Equal(t, 1, toInt(resumeCalls), "resume callback should be called on schedule")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(ctx, func() {}, h)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 2, toInt(pauseCalls), "pause callback should be called on second schedule")
			assert.Equal(t, 2, toInt(resumeCalls), "resume callback should be called on second schedule")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")

		// Stop the scheduler
		cancel()

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 2, toInt(pauseCalls), "pause callback should not be called on shutdown")
			assert.Equal(t, 3, toInt(resumeCalls), "resume callback should be called on shutdown")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")
	})

	t.Run("Running components get stopped on shutdown", func(t *testing.T) {
		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.New(l.Slog())
			h  = scheduler.NewHost()
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
		cs.Schedule(ctx, func() {}, h, component)

		// Wait for the component to start, and then stop our scheduler, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cancel()
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
	})
}

// TestScheduler_Async covers the switch to new components. Schedule is called
// from the Update of an Alloy component, so it must not wait for the switch,
// which can wait on the data path. See
// https://github.com/grafana/alloy/issues/7762.
func TestScheduler_Async(t *testing.T) {
	const timeout = 5 * time.Second

	t.Run("Schedule does not wait for a blocked Shutdown", func(t *testing.T) {
		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.New(l.Slog())
			h  = scheduler.NewHost()
		)
		componenttest.RunInBackground(t, cs.Run)

		release := make(chan struct{})
		old, oldStarted, oldStopped := newBlockingShutdownComponent(release)
		cs.Schedule(t.Context(), func() {}, h, old)
		require.NoError(t, oldStarted.Wait(timeout), "old component did not start")

		// The old component blocks in Shutdown, as a component does when it
		// flushes data to a downstream component that is paused.
		updated := atomic.NewBool(false)
		next, nextStarted, _ := newTriggerComponent()
		requireReturns(t, timeout, func() {
			cs.Schedule(t.Context(), func() { updated.Store(true) }, h, next)
		})
		require.NoError(t, oldStopped.Wait(timeout), "old component was not stopped")
		require.False(t, updated.Load(), "consumers changed before the old component stopped")
		require.False(t, triggered(nextStarted), "new component started before the old component stopped")

		// Unblock the old component. The switch then finishes.
		close(release)
		require.NoError(t, nextStarted.Wait(timeout), "new component did not start")
		require.True(t, updated.Load(), "consumers were not changed")
	})

	t.Run("Only the latest scheduled components are started", func(t *testing.T) {
		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.New(l.Slog())
			h  = scheduler.NewHost()
		)

		// Schedule twice before Run starts.
		skipped, skippedStarted, _ := newTriggerComponent()
		latest, latestStarted, _ := newTriggerComponent()
		cs.Schedule(t.Context(), func() {}, h, skipped)
		cs.Schedule(t.Context(), func() {}, h, latest)

		componenttest.RunInBackground(t, cs.Run)
		require.NoError(t, latestStarted.Wait(timeout), "latest component did not start")
		require.False(t, triggered(skippedStarted), "a replaced set of components was started")
	})

	t.Run("Components scheduled while Run exits are started by the next Run", func(t *testing.T) {
		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.New(l.Slog())
			h  = scheduler.NewHost()
		)

		release := make(chan struct{})
		old, oldStarted, oldStopped := newBlockingShutdownComponent(release)
		cs.Schedule(t.Context(), func() {}, h, old)

		ctx, cancel := context.WithCancel(t.Context())
		runDone := make(chan error, 1)
		go func() { runDone <- cs.Run(ctx) }()
		require.NoError(t, oldStarted.Wait(timeout), "old component did not start")

		// Stop Run. It blocks while it stops the old component.
		cancel()
		require.NoError(t, oldStopped.Wait(timeout), "old component was not stopped")

		// Schedule while Run exits, then let Run exit.
		next, nextStarted, _ := newTriggerComponent()
		requireReturns(t, timeout, func() { cs.Schedule(t.Context(), func() {}, h, next) })
		close(release)
		select {
		case err := <-runDone:
			require.NoError(t, err)
		case <-time.After(timeout):
			require.FailNow(t, "Run did not exit")
		}
		require.False(t, triggered(nextStarted), "component started after Run exited")

		// The next Run starts the latest components.
		componenttest.RunInBackground(t, cs.Run)
		require.NoError(t, nextStarted.Wait(timeout), "new component did not start")
	})
}

// triggered reports whether wt has been triggered. Use it to check that
// something has not happened.
func triggered(wt *util.WaitTrigger) bool {
	return wt.Wait(50*time.Millisecond) == nil
}

// requireReturns fails the test if f does not return within timeout.
func requireReturns(t *testing.T, timeout time.Duration, f func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		require.FailNow(t, "call did not return")
	}
}

// newBlockingShutdownComponent returns a component whose Shutdown blocks until
// release is closed.
func newBlockingShutdownComponent(release <-chan struct{}) (component otelcomponent.Component, started, stopped *util.WaitTrigger) {
	started = util.NewWaitTrigger()
	stopped = util.NewWaitTrigger()

	component = &fakeComponent{
		StartFunc: func(_ context.Context, _ otelcomponent.Host) error {
			started.Trigger()
			return nil
		},
		ShutdownFunc: func(_ context.Context) error {
			stopped.Trigger()
			<-release
			return nil
		},
	}

	return
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
