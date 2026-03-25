package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
)

func TestAuthExtensionScheduler(t *testing.T) {
	t.Run("Scheduled components get started", func(t *testing.T) {
		var (
			l  = util.TestLogger(t)
			h  = scheduler.NewHost(l)
			cs = scheduler.NewAuthExtensionScheduler(l)
		)
		defer cs.Stop()

		component, started, _ := newTriggerComponent()
		cs.Schedule(t.Context(), h, component)
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
	})

	t.Run("Unscheduled components get stopped", func(t *testing.T) {
		var (
			l  = util.TestLogger(t)
			h  = scheduler.NewHost(l)
			cs = scheduler.NewAuthExtensionScheduler(l)
		)
		defer cs.Stop()

		// Schedule our component, which should notify the started and stopped
		// trigger once it starts and stops respectively.
		component, started, stopped := newTriggerComponent()
		cs.Schedule(t.Context(), h, component)

		// Wait for the component to start, and then unschedule all components, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Schedule(t.Context(), h)
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
	})

	t.Run("Running components get stopped on shutdown", func(t *testing.T) {
		var (
			l  = util.TestLogger(t)
			h  = scheduler.NewHost(l)
			cs = scheduler.NewAuthExtensionScheduler(l)
		)

		component, started, stopped := newTriggerComponent()
		cs.Schedule(context.Background(), h, component)

		// Wait for the component to start, and then stop our scheduler, which
		// should cause our running component to terminate.
		require.NoError(t, started.Wait(5*time.Second), "component did not start")
		cs.Stop()
		require.NoError(t, stopped.Wait(5*time.Second), "component did not shutdown")
	})
}
