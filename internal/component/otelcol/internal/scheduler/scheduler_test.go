package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"

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
		cs.Schedule(context.Background(), h, component)
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
		cs.Schedule(context.Background(), h, component)

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(context.Background(), h)
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
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
		cs.Schedule(ctx, h, component)

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
