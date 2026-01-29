package controller_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

func TestScheduler_Synchronize(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stdout)
	t.Run("Can start new jobs", func(t *testing.T) {
		var started, finished sync.WaitGroup
		started.Add(3)
		finished.Add(3)

		runFunc := func(ctx context.Context) error {
			defer finished.Done()
			started.Done()

			<-ctx.Done()
			return nil
		}

		sched := controller.NewScheduler(logger, 1*time.Minute)
		sched.Synchronize([]controller.RunnableNode{
			fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: runFunc}},
			fakeRunnable{ID: "component-b", Component: mockComponent{RunFunc: runFunc}},
			fakeRunnable{ID: "component-c", Component: mockComponent{RunFunc: runFunc}},
		})

		started.Wait()
		require.NoError(t, sched.Close())
		finished.Wait()
	})

	t.Run("Ignores existing jobs", func(t *testing.T) {
		var started sync.WaitGroup
		started.Add(1)

		runFunc := func(ctx context.Context) error {
			started.Done()
			<-ctx.Done()
			return nil
		}

		sched := controller.NewScheduler(logger, 1*time.Minute)

		for i := 0; i < 10; i++ {
			// If a new runnable is created, runFunc will panic since the WaitGroup
			// only supports 1 goroutine.
			sched.Synchronize([]controller.RunnableNode{
				fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: runFunc}},
			})
		}

		started.Wait()
		require.NoError(t, sched.Close())
	})

	t.Run("Runnables which no longer exist are shutdown before new ones are created", func(t *testing.T) {
		var started, finished sync.WaitGroup
		started.Add(2)

		var lock sync.Mutex

		basicRun := func(ctx context.Context) error {
			defer finished.Done()
			started.Done()
			<-ctx.Done()
			return nil
		}

		sharedResourceRun := func(ctx context.Context) error {
			defer finished.Done()
			started.Done()

			if !lock.TryLock() {
				t.Fatal("failed to claim lock - already held by another component")
				return nil
			}
			defer lock.Unlock()
			<-ctx.Done()
			return nil
		}

		sched := controller.NewScheduler(logger, 1*time.Minute)

		comp1 := fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: sharedResourceRun}}
		comp2 := fakeRunnable{ID: "component-b", Component: mockComponent{RunFunc: basicRun}}
		comp3 := fakeRunnable{ID: "component-c", Component: mockComponent{RunFunc: sharedResourceRun}}

		sched.Synchronize([]controller.RunnableNode{comp1, comp2})
		started.Wait()

		started.Add(1)
		finished.Add(1)
		sched.Synchronize([]controller.RunnableNode{comp2, comp3})
		started.Wait()
		finished.Wait()

		finished.Add(2)
		require.NoError(t, sched.Close())
		finished.Wait()
	})

	t.Run("Timeout allows new tasks to start but waits for old tasks to finish", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			oldTaskExited := false
			newTaskStarted := false

			// Old task that takes a long time to stop
			slowStop := func(ctx context.Context) error {
				<-ctx.Done()

				// Simulate slow shutdown
				time.Sleep(2 * controller.TaskShutdownWarningTimeout)
				oldTaskExited = true
				return nil
			}

			// New task
			basicRun := func(ctx context.Context) error {
				newTaskStarted = true
				<-ctx.Done()
				return nil
			}

			sched := controller.NewScheduler(logger, 5*time.Minute)

			// Start component-a with slow stop behavior
			comp1 := fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: slowStop}}
			err := sched.Synchronize([]controller.RunnableNode{comp1})
			require.NoError(t, err)

			// Replace with component-b
			// This should timeout waiting for component-a, start component-b anyway,
			// but not return until component-a fully exits
			comp2 := fakeRunnable{ID: "component-b", Component: mockComponent{RunFunc: basicRun}}

			syncDone := make(chan struct{})
			go func() {
				err := sched.Synchronize([]controller.RunnableNode{comp2})
				require.NoError(t, err)
				close(syncDone)
			}()

			// Wait past the timeout for new task to start
			time.Sleep(controller.TaskShutdownWarningTimeout + 1*time.Second)

			require.True(t, newTaskStarted, "new task should have started after timeout")
			require.False(t, oldTaskExited, "old task should still be running")

			select {
			case <-syncDone:
				t.Error("Synchronize returned before old task finished")
			default:
			}

			// Wait for old task to finish
			time.Sleep(2 * time.Minute)

			select {
			case <-syncDone:
			default:
				t.Error("Synchronize should have returned after old task finished")
			}

			require.True(t, oldTaskExited, "old task should have exited")
			require.True(t, newTaskStarted, "new task should still be running")

			require.NoError(t, sched.Close())
		})
	})
}

type fakeRunnable struct {
	ID        string
	Component component.Component
}

var _ controller.RunnableNode = fakeRunnable{}

func (fr fakeRunnable) NodeID() string                 { return fr.ID }
func (fr fakeRunnable) Run(ctx context.Context) error  { return fr.Component.Run(ctx) }
func (fr fakeRunnable) Block() *ast.BlockStmt          { return nil }
func (fr fakeRunnable) Evaluate(scope *vm.Scope) error { return nil }
func (fr fakeRunnable) UpdateBlock(b *ast.BlockStmt)   {}

type mockComponent struct {
	RunFunc    func(ctx context.Context) error
	UpdateFunc func(newConfig component.Arguments) error
}

var _ component.Component = (*mockComponent)(nil)

func (mc mockComponent) Run(ctx context.Context) error              { return mc.RunFunc(ctx) }
func (mc mockComponent) Update(newConfig component.Arguments) error { return mc.UpdateFunc(newConfig) }

func TestScheduler_TaskTimeoutLogging(t *testing.T) {
	// Temporarily modify timeout values for testing
	originalWarningTimeout := controller.TaskShutdownWarningTimeout
	controller.TaskShutdownWarningTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		controller.TaskShutdownWarningTimeout = originalWarningTimeout
	})

	// Create a buffer to capture log output
	var logBuffer bytes.Buffer
	logger := log.NewLogfmtLogger(&logBuffer)

	var started sync.WaitGroup
	started.Add(1)

	// Create a component that will block and not respond to context cancellation
	runFunc := func(ctx context.Context) error {
		started.Done()
		// Block indefinitely, ignoring context cancellation
		// Use a long sleep to simulate a component that doesn't respond to cancellation
		time.Sleep(1 * time.Second)
		return nil
	}

	sched := controller.NewScheduler(logger, 150*time.Millisecond)

	// Start a component
	err := sched.Synchronize([]controller.RunnableNode{
		fakeRunnable{ID: "blocking-component", Component: mockComponent{RunFunc: runFunc}},
	})
	require.NoError(t, err)
	started.Wait()

	// Remove the component, which should trigger the timeout behavior. This will block until the component exits.
	err = sched.Synchronize([]controller.RunnableNode{})
	require.NoError(t, err)

	logOutput := logBuffer.String()
	t.Logf("actual log output:\n%s", logOutput)

	// Should contain warning message
	require.Contains(t, logOutput, "task shutdown is taking longer than expected")
	require.Contains(t, logOutput, "level=warn")

	// Should contain error message
	require.Contains(t, logOutput, "task shutdown deadline exceeded")
	require.Contains(t, logOutput, "level=error")

	require.NoError(t, sched.Close())
}
