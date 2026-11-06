package lazyconsumer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/atomic"
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
	for i := 0; i < routines; i++ {
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
	for i := 0; i < routines; i++ {
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

	for i := 0; i < runs; i++ {
		workChan <- struct{}{}
	}
	cancel()

	allDone.Wait()

	// Should not be paused as last call will always be c.Resume()
	require.False(t, c.IsPaused())
}

func Test_PauseWaitsForInFlightConsume(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())

	method := map[string]func(c *Consumer){
		"ConsumeTraces": func(c *Consumer) {
			_ = c.ConsumeTraces(componenttest.TestContext(t), ptrace.NewTraces())
		},
		"ConsumeMetrics": func(c *Consumer) {
			_ = c.ConsumeMetrics(componenttest.TestContext(t), pmetric.NewMetrics())
		},
		"ConsumeLogs": func(c *Consumer) {
			_ = c.ConsumeLogs(componenttest.TestContext(t), plog.NewLogs())
		},
	}

	for name, consume := range method {
		t.Run(name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			block := func() {
				close(entered)
				<-release
			}

			c := New(componenttest.TestContext(t), "test_component")
			setConsumers(t, c, block)

			// Start a Consume* call and wait until it is forwarding data
			consumeDone := make(chan struct{})
			go func() {
				consume(c)
				close(consumeDone)
			}()
			waitFor(t, entered, "the Consume* call to reach the consumer")

			// Pause must not return while that call is still in progress
			pauseDone := make(chan struct{})
			go func() {
				c.Pause()
				close(pauseDone)
			}()
			select {
			case <-pauseDone:
				t.Fatal("Pause returned while a Consume* call was still forwarding data")
			case <-time.After(100 * time.Millisecond):
			}

			// Once the call returns, Pause returns too
			close(release)
			waitFor(t, consumeDone, "the Consume* call to return")
			waitFor(t, pauseDone, "Pause to return")
			require.True(t, c.IsPaused())
			c.Resume()
		})
	}
}

// Test_NoConsumeAfterPause stands in for a component being shut down while
// it is paused: no call may reach it between Pause and Resume.
func Test_NoConsumeAfterPause(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())
	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	routines := 8
	runs := 500

	var stopped atomic.Bool
	var violations atomic.Int64
	c := New(componenttest.TestContext(t), "test_component")
	setConsumers(t, c, func() {
		if stopped.Load() {
			violations.Add(1)
		}
		time.Sleep(10 * time.Microsecond)
		if stopped.Load() {
			violations.Add(1)
		}
	})

	allDone := sync.WaitGroup{}
	for i := 0; i < routines; i++ {
		allDone.Add(1)
		go func() {
			defer allDone.Done()
			for ctx.Err() == nil {
				_ = c.ConsumeTraces(ctx, ptrace.NewTraces())
				_ = c.ConsumeMetrics(ctx, pmetric.NewMetrics())
				_ = c.ConsumeLogs(ctx, plog.NewLogs())
			}
		}()
	}

	for i := 0; i < runs; i++ {
		c.Pause()
		stopped.Store(true)
		time.Sleep(20 * time.Microsecond)
		stopped.Store(false)
		c.Resume()
	}

	cancel()
	allDone.Wait()
	require.Zero(t, violations.Load(), "Consume* calls reached the consumer after Pause returned")
}

// Test_PauseCancelsStuckConsume covers a downstream that blocks until its
// context ends, like an exporter retrying against a failing backend: Pause
// cancels the call after drainTimeout instead of waiting for it indefinitely.
func Test_PauseCancelsStuckConsume(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())

	entered := make(chan struct{}, 1)
	c := New(componenttest.TestContext(t), "test_component")
	c.drainTimeout = 50 * time.Millisecond
	setConsumersFunc(t, c, func(ctx context.Context) error {
		entered <- struct{}{}
		<-ctx.Done()
		return context.Cause(ctx)
	})

	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- c.ConsumeTraces(componenttest.TestContext(t), ptrace.NewTraces())
	}()
	waitFor(t, entered, "the Consume* call to reach the consumer")

	start := time.Now()
	c.Pause()
	require.GreaterOrEqual(t, time.Since(start), c.drainTimeout, "Pause should give in-flight calls drainTimeout to return")
	require.ErrorIs(t, <-consumeErr, errPaused)

	// Calls made after Resume get a context that isn't cancelled.
	c.Resume()
	setConsumersFunc(t, c, func(ctx context.Context) error {
		return ctx.Err()
	})
	require.NoError(t, c.ConsumeTraces(componenttest.TestContext(t), ptrace.NewTraces()))
}

// Test_PauseDoesNotWaitForeverOnCallsIgnoringContext covers a downstream that
// ignores its context: Pause returns after twice drainTimeout regardless.
func Test_PauseDoesNotWaitForeverOnCallsIgnoringContext(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())

	entered := make(chan struct{})
	release := make(chan struct{})
	c := New(componenttest.TestContext(t), "test_component")
	c.drainTimeout = 50 * time.Millisecond
	setConsumers(t, c, func() {
		close(entered)
		<-release
	})

	consumeDone := make(chan struct{})
	go func() {
		_ = c.ConsumeLogs(componenttest.TestContext(t), plog.NewLogs())
		close(consumeDone)
	}()
	waitFor(t, entered, "the Consume* call to reach the consumer")

	pauseDone := make(chan struct{})
	go func() {
		c.Pause()
		close(pauseDone)
	}()
	waitFor(t, pauseDone, "Pause to give up on the call")
	require.True(t, c.IsPaused())

	close(release)
	waitFor(t, consumeDone, "the Consume* call to return")
	c.Resume()
}

// Test_PausedConsumeReturnsWhenContextEnds covers a consumer that stays paused
// after its components shut down: the calls waiting on it return once the
// consumer's context ends instead of blocking forever.
func Test_PausedConsumeReturnsWhenContextEnds(t *testing.T) {
	goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	c := NewPaused(ctx, "test_component")
	setConsumers(t, c, func() {
		t.Error("data reached the consumer of a stopped component")
	})

	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- c.ConsumeMetrics(componenttest.TestContext(t), pmetric.NewMetrics())
	}()
	select {
	case err := <-consumeErr:
		t.Fatalf("Consume* returned while paused: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-consumeErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Consume* did not return after the consumer's context ended")
	}
}

// setConsumers sets consumers for all signals that call fn for each request.
func setConsumers(t *testing.T, c *Consumer, fn func()) {
	t.Helper()
	setConsumersFunc(t, c, func(context.Context) error {
		fn()
		return nil
	})
}

// setConsumersFunc sets consumers for all signals that return fn's result for
// each request.
func setConsumersFunc(t *testing.T, c *Consumer, fn func(ctx context.Context) error) {
	t.Helper()
	traces, err := otelconsumer.NewTraces(func(ctx context.Context, _ ptrace.Traces) error {
		return fn(ctx)
	})
	require.NoError(t, err)
	metrics, err := otelconsumer.NewMetrics(func(ctx context.Context, _ pmetric.Metrics) error {
		return fn(ctx)
	})
	require.NoError(t, err)
	logs, err := otelconsumer.NewLogs(func(ctx context.Context, _ plog.Logs) error {
		return fn(ctx)
	})
	require.NoError(t, err)
	c.SetConsumers(traces, metrics, logs)
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}
