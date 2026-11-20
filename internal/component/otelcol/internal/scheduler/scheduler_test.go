package scheduler_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
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

		// Shutdown pauses before stopping the components and doesn't resume, so no data reaches them afterwards.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Equal(t, 4, toInt(pauseCalls), "pause callback should be called on shutdown")
			assert.Equal(t, 3, toInt(resumeCalls), "resume callback should not be called on shutdown")
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

	t.Run("Shutdown pauses before stopping components", func(t *testing.T) {
		var (
			mut    sync.Mutex
			events []string
			record = func(event string) {
				mut.Lock()
				defer mut.Unlock()
				events = append(events, event)
			}
			l  = util.TestAlloyLogger(t)
			cs = scheduler.NewWithPauseCallbacks(
				l.Slog(),
				func() { record("pause") },
				func() { record("resume") },
			)
			h = scheduler.NewHost()
		)

		ctx, cancel := context.WithCancel(componenttest.TestContext(t))
		defer cancel()

		runDone := make(chan struct{})
		go func() {
			defer close(runDone)
			err := cs.Run(ctx)
			require.NoError(t, err)
		}()

		component := &fakeComponent{
			StartFunc: func(_ context.Context, _ otelcomponent.Host) error {
				record("start")
				return nil
			},
			ShutdownFunc: func(_ context.Context) error {
				record("shutdown")
				return nil
			},
		}
		cs.Schedule(ctx, func() {}, h, component)

		// Wait for the resume that follows the start, which Run or Schedule makes depending on which goes first.
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			mut.Lock()
			defer mut.Unlock()
			i := slices.Index(events, "start")
			assert.True(c, i >= 0 && slices.Contains(events[i:], "resume"))
		}, 5*time.Second, 10*time.Millisecond, "component did not start and resume")

		// Only look at what the shutdown does.
		mut.Lock()
		events = nil
		mut.Unlock()

		cancel()
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return")
		}

		mut.Lock()
		defer mut.Unlock()
		require.Equal(t, []string{"pause", "shutdown"}, events, "shutdown should pause, then stop the components, and not resume")
	})
}

// TestSchedulerWithLazyConsumer runs the scheduler with a lazy consumer's Pause and Resume as its callbacks, as the
// otelcol processor, exporter and connector components do, and checks that no call reaches a stopped component.
func TestSchedulerWithLazyConsumer(t *testing.T) {
	for _, tc := range []struct {
		name string
		// stop replaces or removes the first component.
		stop func(cancelRun context.CancelFunc, schedule func(id string))
	}{
		{
			name: "Reload",
			stop: func(_ context.CancelFunc, schedule func(id string)) { schedule("second") },
		},
		{
			name: "Shutdown",
			stop: func(cancelRun context.CancelFunc, _ func(id string)) { cancelRun() },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				l        = util.TestAlloyLogger(t)
				ctx      = componenttest.TestContext(t)
				consumer = lazyconsumer.NewPaused(ctx, "test_component", l.Slog())
				cs       = scheduler.NewWithPauseCallbacks(l.Slog(), consumer.Pause, consumer.Resume)
				h        = scheduler.NewHost()

				mut     sync.Mutex
				stopped = map[string]bool{}
				entered = make(chan string, 1)
				release = make(chan struct{})
			)

			// schedule runs a component whose consumer blocks the first call until release is closed.
			schedule := func(id string) {
				traces, err := otelconsumer.NewTraces(func(context.Context, ptrace.Traces) error {
					mut.Lock()
					isStopped := stopped[id]
					mut.Unlock()
					if isStopped {
						t.Errorf("data reached component %q after it was stopped", id)
					}
					select {
					case entered <- id:
						<-release
					default:
					}
					return nil
				})
				require.NoError(t, err)
				component := &fakeComponent{ShutdownFunc: func(context.Context) error {
					mut.Lock()
					defer mut.Unlock()
					stopped[id] = true
					return nil
				}}
				cs.Schedule(ctx, func() { consumer.SetConsumers(traces, nil, nil) }, h, component)
			}

			runCtx, cancelRun := context.WithCancel(ctx)
			defer cancelRun()
			runDone := make(chan struct{})
			go func() {
				defer close(runDone)
				assert.NoError(t, cs.Run(runCtx))
			}()
			schedule("first")
			require.Eventually(t, func() bool { return !consumer.IsPaused() }, 5*time.Second, time.Millisecond)

			// A call is in flight to the first component.
			consumeErr := make(chan error, 1)
			go func() {
				consumeErr <- consumer.ConsumeTraces(ctx, ptrace.NewTraces())
			}()
			select {
			case id := <-entered:
				require.Equal(t, "first", id)
			case <-time.After(5 * time.Second):
				t.Fatal("the call did not reach the first component")
			}

			stopDone := make(chan struct{})
			go func() {
				tc.stop(cancelRun, schedule)
				close(stopDone)
			}()

			// The first component isn't stopped while the call is in flight.
			time.Sleep(100 * time.Millisecond)
			mut.Lock()
			require.False(t, stopped["first"], "the component was stopped while a call was in flight")
			mut.Unlock()

			close(release)
			require.NoError(t, <-consumeErr)
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				mut.Lock()
				defer mut.Unlock()
				assert.True(c, stopped["first"])
			}, 5*time.Second, time.Millisecond, "the first component was not stopped")

			select {
			case <-stopDone:
			case <-time.After(5 * time.Second):
				t.Fatal("stopping the first component did not finish")
			}
			cancelRun()
			select {
			case <-runDone:
			case <-time.After(5 * time.Second):
				t.Fatal("Run did not return")
			}
		})
	}
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
