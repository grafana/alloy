package controller_test

import (
	"bytes"
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

func TestScheduler_Synchronize(t *testing.T) {
	logger := log.NewNopLogger()
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

		g := &dag.Graph{}

		g.Add(&fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: runFunc}})
		g.Add(&fakeRunnable{ID: "component-b", Component: mockComponent{RunFunc: runFunc}})
		g.Add(&fakeRunnable{ID: "component-c", Component: mockComponent{RunFunc: runFunc}})

		sched := controller.NewScheduler(logger, 1*time.Minute)
		sched.Synchronize(g)

		started.Wait()
		sched.Stop()
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
		g := &dag.Graph{}
		g.Add(&fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: runFunc}})

		for i := 0; i < 10; i++ {
			// If a new runnable is created, runFunc will panic since the WaitGroup
			// only supports 1 goroutine.
			sched.Synchronize(g)
		}

		started.Wait()
		sched.Stop()
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
		g := &dag.Graph{}
		g.Add(&fakeRunnable{ID: "component-a", Component: mockComponent{RunFunc: sharedResourceRun}})
		g.Add(&fakeRunnable{ID: "component-b", Component: mockComponent{RunFunc: basicRun}})

		sched.Synchronize(g)
		started.Wait()

		started.Add(1)
		finished.Add(1)
		g = &dag.Graph{}
		g.Add(&fakeRunnable{ID: "component-b", Component: mockComponent{RunFunc: basicRun}})
		g.Add(&fakeRunnable{ID: "component-c", Component: mockComponent{RunFunc: sharedResourceRun}})
		sched.Synchronize(g)
		started.Wait()
		finished.Wait()

		finished.Add(2)
		sched.Stop()
		finished.Wait()
	})

	t.Run("Task shutdown deadline logs warnings and errors", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Create a thread-safe buffer to capture log output
			var logBuffer syncBuffer
			logger := log.NewLogfmtLogger(&logBuffer)

			runFunc := func(ctx context.Context) error {
				<-ctx.Done()
				// Block indefinitely, ignoring context cancellation
				time.Sleep(3 * time.Minute)
				return nil
			}

			sched := controller.NewScheduler(logger, 2*time.Minute)
			g := &dag.Graph{}
			g.Add(&fakeRunnable{ID: "blocking-component", Component: mockComponent{RunFunc: runFunc}})

			// Start a component
			err := sched.Synchronize(g)
			require.NoError(t, err)

			syncDone := make(chan struct{})
			go func() {
				g := &dag.Graph{}
				err := sched.Synchronize(g)
				require.NoError(t, err)
				close(syncDone)
			}()

			time.Sleep(controller.TaskShutdownWarningTimeout + 1*time.Second)

			// Should have warning message
			logOutput := logBuffer.String()
			require.Contains(t, logOutput, "task shutdown is taking longer than expected")
			require.Contains(t, logOutput, "level=warn")

			// Wait past the shutdown deadline
			time.Sleep(2*time.Minute + 1*time.Second)

			// Should have error message
			logOutput = logBuffer.String()
			require.Contains(t, logOutput, "task shutdown deadline exceeded")
			require.Contains(t, logOutput, "level=error")

			// Synchronize should have returned
			select {
			case <-syncDone:
				// Good
			default:
				t.Error("Synchronize should have returned after deadline")
			}

			sched.Stop()

			// Sleep long enough to let the runFunc exit to preventing a synctest panic
			time.Sleep(time.Minute)
		})
	})

	t.Run("Tasks are shutdown from roots to leaves within each subgraph", func(t *testing.T) {
		var stopOrder []string

		edges := []edge{
			{From: "g1_root1", To: "g1_mid1"}, {From: "g1_root1", To: "g1_leaf2"}, {From: "g1_root1", To: "g1_mid2"},
			{From: "g1_mid1", To: "g1_leaf1"}, {From: "g1_mid2", To: "g1_leaf1"},
			{From: "g2_root1", To: "g2_leaf1"},
		}

		g := buildGraphFromEdges(edges, &stopOrder)
		sched := controller.NewScheduler(logger, 1*time.Minute)
		err := sched.Synchronize(g)
		require.NoError(t, err)
		sched.Stop()

		indexOf := func(slice []string, id string) int {
			return slices.IndexFunc(slice, func(n string) bool {
				return n == id
			})
		}

		for _, e := range g.Edges() {
			from := indexOf(stopOrder, e.From.NodeID())
			to := indexOf(stopOrder, e.To.NodeID())
			require.Less(t, from, to)
		}
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

// syncBuffer wraps bytes.Buffer with mutex for thread-safe reads and writes
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

func buildGraphFromEdges(edges []edge, stopOrder *[]string) *dag.Graph {
	set := make(map[string]struct{})
	for _, e := range edges {
		set[e.From] = struct{}{}
		set[e.To] = struct{}{}
	}

	nodes := make(map[string]*fakeRunnable, len(set))
	for id := range set {
		nodes[id] = &fakeRunnable{
			ID: id,
			Component: mockComponent{RunFunc: func(ctx context.Context) error {
				<-ctx.Done()
				*stopOrder = append(*stopOrder, id)
				return nil
			}},
		}
	}
	g := &dag.Graph{}
	for _, n := range nodes {
		g.Add(n)
	}
	for _, e := range edges {
		g.AddEdge(dag.Edge{From: nodes[e.From], To: nodes[e.To]})
	}
	return g
}
