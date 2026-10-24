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

		// The Run function starts the components. They should be paused and then resumed.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 1, toInt(pauseCalls), "pause callbacks should be called on run")
			assert.Equal(t, 1, toInt(resumeCalls), "resume callback should be called on run")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		// Schedule our component, which should notify the started and stopped
		// trigger once it starts and stops respectively.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(ctx, func() {}, h, component)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 2, toInt(pauseCalls), "pause callbacks should be called on schedule")
			assert.Equal(t, 2, toInt(resumeCalls), "resume callback should be called on schedule")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(ctx, func() {}, h)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 3, toInt(pauseCalls), "pause callback should be called on second schedule")
			assert.Equal(t, 3, toInt(resumeCalls), "resume callback should be called on second schedule")
		}, 5*time.Second, 10*time.Millisecond, "pause/resume callbacks not called correctly")

		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")

		// Stop the scheduler
		cancel()

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 3, toInt(pauseCalls), "pause callback should not be called on shutdown")
			assert.Equal(t, 4, toInt(resumeCalls), "resume callback should be called on shutdown")
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

	// TestShutdownTimeout_HotReload verifies that when a component's Shutdown()
	// blocks longer than shutdownTimeout, Schedule() still returns within a
	// bounded time and the new component is successfully started. This is a
	// regression test for https://github.com/grafana/alloy/issues/6622 where a
	// blocking Shutdown (e.g. a gRPC server draining long-lived connections)
	// caused /-/reload to hang indefinitely.
	t.Run("Schedule returns promptly when old component Shutdown blocks (shutdownTimeout)", func(t *testing.T) {
		const shutdownTimeout = 200 * time.Millisecond

		var (
			l  = util.TestAlloyLogger(t)
			cs = scheduler.NewWithPauseCallbacks(l.Slog(), func() {}, func() {})
			h  = scheduler.NewHost()
		)
		// Override the default shutdown timeout to something short for this test.
		cs.SetShutdownTimeout(shutdownTimeout)

		// Run our scheduler in the background.
		go func() {
			err := cs.Run(componenttest.TestContext(t))
			require.NoError(t, err)
		}()

		// First, schedule a component that blocks on Shutdown (simulating a gRPC
		// server with active long-lived connections configured with keepalive params).
		blockingStarted := util.NewWaitTrigger()
		shutdownCalled := util.NewWaitTrigger()
		blockingComponent := &fakeComponent{
			StartFunc: func(_ context.Context, _ otelcomponent.Host) error {
				blockingStarted.Trigger()
				return nil
			},
			ShutdownFunc: func(ctx context.Context) error {
				// Signal that Shutdown was called, then block until the context
				// provided by the scheduler (with shutdownTimeout) expires.
				shutdownCalled.Trigger()
				<-ctx.Done()
				return ctx.Err()
			},
		}
		cs.Schedule(t.Context(), func() {}, h, blockingComponent)

		// Give the scheduler's Run() goroutine time to start the blocking component.
		require.NoError(t, blockingStarted.Wait(5*time.Second))
		// Wait just a bit to ensure Run() has fully started the component.
		time.Sleep(50 * time.Millisecond)

		// Hot-reload: schedule a new component. This calls Shutdown on the
		// blocking component. With the timeout, Schedule must not block past
		// shutdownTimeout.
		newComponent, newStarted, _ := newTriggerComponent()

		start := time.Now()
		cs.Schedule(t.Context(), func() {}, h, newComponent)
		elapsed := time.Since(start)

		// Schedule should return within roughly shutdownTimeout + overhead,
		// NOT after an indefinite wait.
		assert.Less(t, elapsed, shutdownTimeout+2*time.Second,
			"Schedule() took too long; Shutdown() may be blocking the hot-reload path")

		// The new component must be started even though the old one's Shutdown blocked.
		require.NoError(t, newStarted.Wait(5*time.Second), "new component did not start after hot-reload")

		// Shutdown of the old component should have been called.
		require.NoError(t, shutdownCalled.Wait(5*time.Second), "old component Shutdown was not called")
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
