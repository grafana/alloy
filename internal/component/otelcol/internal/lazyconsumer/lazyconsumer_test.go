package lazyconsumer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/runtime/componenttest"
)

func Test_PauseAndResume(t *testing.T) {
	c := New(componenttest.TestContext(t), "test_component")
	require.False(t, c.IsPaused())
	c.Pause()
	require.True(t, c.IsPaused())
	c.Resume()
	require.False(t, c.IsPaused())
}

func Test_NewPaused(t *testing.T) {
	c := NewPaused(componenttest.TestContext(t), "test_component")
	require.True(t, c.IsPaused())
	c.Resume()
	require.False(t, c.IsPaused())
}

func Test_PauseResume_MultipleCalls(t *testing.T) {
	c := New(componenttest.TestContext(t), "test_component")
	require.False(t, c.IsPaused())
	c.Pause()
	c.Pause()
	c.Pause()
	require.True(t, c.IsPaused())
	c.Resume()
	c.Resume()
	c.Resume()
	require.False(t, c.IsPaused())
}

func Test_ConsumeWaitsForResume(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())
	c := NewPaused(componenttest.TestContext(t), "test_component")
	require.True(t, c.IsPaused())

	method := map[string]func(){
		"ConsumeTraces": func() {
			_ = c.ConsumeTraces(componenttest.TestContext(t), ptrace.NewTraces())
		},
		"ConsumeMetrics": func() {
			_ = c.ConsumeMetrics(componenttest.TestContext(t), pmetric.NewMetrics())
		},
		"ConsumeLogs": func() {
			_ = c.ConsumeLogs(componenttest.TestContext(t), plog.NewLogs())
		},
	}

	for name, fn := range method {
		t.Run(name, func(t *testing.T) {
			c.Pause()
			require.True(t, c.IsPaused())

			started := make(chan struct{})
			finished := make(chan struct{})

			// Start goroutine that attempts to run Consume* method
			go func() {
				started <- struct{}{}
				fn()
				finished <- struct{}{}
			}()

			// Wait to be started
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("consumer goroutine never started")
			}

			// Wait for a bit to ensure the consumer is blocking on Consume* function
			select {
			case <-finished:
				t.Fatal("consumer should not have finished yet - it's paused")
			case <-time.After(100 * time.Millisecond):
			}

			// Resume the consumer and verify the Consume* function unblocked
			c.Resume()
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("consumer should have finished after resuming")
			}
		})
	}
}

func Test_PauseResume_Multithreaded(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())
	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	runs := 500
	routines := 5
	allDone := sync.WaitGroup{}

	c := NewPaused(componenttest.TestContext(t), "test_component")
	require.True(t, c.IsPaused())

	// Run goroutines that constantly try to call Consume* methods
	for range routines {
		allDone.Add(1)
		go func() {
			for {
				select {
				case <-ctx.Done():
					allDone.Done()
					return
				default:
					_ = c.ConsumeLogs(ctx, plog.NewLogs())
					_ = c.ConsumeMetrics(ctx, pmetric.NewMetrics())
					_ = c.ConsumeTraces(ctx, ptrace.NewTraces())
				}
			}
		}()
	}

	// Run goroutines that Pause and then Resume in parallel.
	// In particular, this verifies we can call .Pause() and .Resume() on an already paused or already resumed consumer.
	workChan := make(chan struct{}, routines)
	for range routines {
		allDone.Add(1)
		go func() {
			for {
				select {
				case <-workChan:
					c.Pause()
					c.Resume()
				case <-ctx.Done():
					allDone.Done()
					return
				}
			}
		}()
	}

	for range runs {
		workChan <- struct{}{}
	}
	cancel()

	allDone.Wait()

	// Should not be paused as last call will always be c.Resume()
	require.False(t, c.IsPaused())
}
