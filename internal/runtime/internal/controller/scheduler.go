package controller

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

var (
	// TaskShutdownWarningTimeout is the duration after which a warning is logged
	// when a task is taking too long to shut down.
	TaskShutdownWarningTimeout = time.Minute
)

// RunnableNode is any BlockNode which can also be run.
type RunnableNode interface {
	BlockNode
	Run(ctx context.Context) error
}

// Scheduler runs components.
type Scheduler struct {
	running              sync.WaitGroup
	logger               log.Logger
	taskShutdownDeadline time.Duration

	tasksMut sync.Mutex
	tasks    map[string]*task
}

// NewScheduler creates a new Scheduler. Call Synchronize to manage the set of
// components which are running.
//
// Call Stop to stop the Scheduler and all running components.
func NewScheduler(logger log.Logger, taskShutdownDeadline time.Duration) *Scheduler {
	return &Scheduler{
		logger:               logger,
		taskShutdownDeadline: taskShutdownDeadline,

		tasks: make(map[string]*task),
	}
}

// Synchronize adjusts the set of running components based on the provided
// graph in the following way,
//
// 1. Nodes already managed by the scheduler will be unchanged.
// 2. Nodes which are no longer present will be told to shutdown.
// 3. Nodes will be given 1 minute to shutdown before new nodes are created.
// 4. Wait for any remaining nodes to shutdown
//
// Nodes are shutdown first to ensure any shared resources, such as ports,
// are allowed to be freed before new nodes are scheduled. As a means to avoid,
// long stretches of downtime we give this a 1 minute timeout.
//
// Tasks are stopped from roots to leaves and started from leaves to roots.
//
// Existing components will be restarted if they stopped since the previous
// call to Synchronize.
//
// Synchronize is not goroutine safe and should not be called concurrently.
func (s *Scheduler) Synchronize(g *dag.Graph) error {
	desiredTasks := desiredTasksFromGraph(g)

	toStop := make(map[int][]*task)
	s.tasksMut.Lock()
	for id, t := range s.tasks {
		if dt, keep := desiredTasks[id]; keep {
			t.groupID = dt.groupID
			t.rank = dt.rank
			continue
		}
		toStop[t.groupID] = append(toStop[t.groupID], t)
	}
	s.tasksMut.Unlock()

	var stopping sync.WaitGroup
	for _, group := range toStop {
		stopping.Go(func() {
			for _, t := range stopOrder(group) {
				t.Stop()
			}
		})
	}

	stopping.Wait()

	s.tasksMut.Lock()
	toStart := make(map[int][]*task)

	// Launch new runnables that have appeared.
	for id, t := range desiredTasks {
		if _, exist := s.tasks[id]; exist {
			continue
		}

		s.running.Add(1)
		task := newTask(t.groupID, t.rank, taskOptions{
			runnable: t.runnable,
			onDone: func(err error) {
				defer s.running.Done()
				if err != nil {
					level.Error(s.logger).Log("msg", "node exited with error", "node", id, "err", err)
				} else {
					level.Info(s.logger).Log("msg", "node exited without error", "node", id)
				}

				s.tasksMut.Lock()
				defer s.tasksMut.Unlock()
				delete(s.tasks, id)
			},
			logger:               log.With(s.logger, "taskID", id),
			taskShutdownDeadline: s.taskShutdownDeadline,
		})

		s.tasks[id] = task
		toStart[task.groupID] = append(toStart[task.groupID], task)
	}
	s.tasksMut.Unlock()

	var starting sync.WaitGroup

	for _, group := range toStart {
		starting.Go(func() {
			for _, t := range startOrder(group) {
				t.Start()
			}
		})
	}

	starting.Wait()
	return nil
}

// Stop stops the Scheduler and returns after all running goroutines have
// exited.
func (s *Scheduler) Stop() {
	s.tasksMut.Lock()

	toStop := make(map[int][]*task)
	for _, t := range s.tasks {
		toStop[t.groupID] = append(toStop[t.groupID], t)
	}
	s.tasksMut.Unlock()

	for _, group := range toStop {
		go func() {
			// NOTE: we cannot hold lock when calling Stop because onDone will mutate running tasks.
			for _, t := range stopOrder(group) {
				t.Stop()
			}
		}()
	}

	s.running.Wait()
}

// task is a scheduled runnable.
type task struct {
	groupID int
	rank    int

	ctx      context.Context
	cancel   context.CancelFunc
	exited   chan struct{}
	opts     taskOptions
	doneOnce sync.Once
}

type taskOptions struct {
	runnable             RunnableNode
	onDone               func(error)
	logger               log.Logger
	taskShutdownDeadline time.Duration
}

// newTask creates and starts a new task.
func newTask(groupID, rank int, opts taskOptions) *task {
	t := &task{
		groupID: groupID,
		rank:    rank,
		opts:    opts,
	}

	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.exited = make(chan struct{})

	return t
}

func (t *task) Start() {
	level.Debug(t.opts.logger).Log("msg", "Starting task", "id", t.opts.runnable.NodeID())

	go func() {
		err := t.opts.runnable.Run(t.ctx)
		// NOTE: make sure we call cancel here so if the runnable
		// exit unexpectedly we clean up resources.
		t.cancel()
		close(t.exited)
		t.doneOnce.Do(func() {
			t.opts.onDone(err)
		})
	}()
}

func (t *task) Stop() {
	level.Debug(t.opts.logger).Log("msg", "Stopping task", "id", t.opts.runnable.NodeID())
	t.cancel()

	deadlineDuration := t.opts.taskShutdownDeadline
	if deadlineDuration == 0 {
		deadlineDuration = time.Hour * 24 * 365 * 100 // infinite timeout ~= 100 years
	}

	deadlineCtx, deadlineCancel := context.WithTimeout(context.Background(), deadlineDuration)
	defer deadlineCancel()

	for {
		select {
		case <-t.exited:
			return // Task exited normally.
		case <-time.After(TaskShutdownWarningTimeout):
			level.Warn(t.opts.logger).Log("msg", "task shutdown is taking longer than expected")
		case <-deadlineCtx.Done():
			t.doneOnce.Do(func() {
				t.opts.onDone(fmt.Errorf("task shutdown deadline exceeded"))
			})
			return // Task took too long to exit, don't wait.
		}
	}
}

// desiredTask describes a runnable to be scheduled.
type desiredTask struct {
	// rank defines order, start tasks in ascending rank order, stop in descending rank order
	rank int
	// groupID is ephemeral and can change between Synchronize calls
	groupID  int
	runnable RunnableNode
}

func desiredTasksFromGraph(g *dag.Graph) map[string]desiredTask {
	var (
		desiredTasks = make(map[string]desiredTask, g.Len())
		components   = dag.WeaklyConnectedComponents(g, dag.FilterLeavesFunc)
	)

	for groupID, leaves := range components {
		var rank int
		_ = dag.WalkTopological(g, leaves, func(n dag.Node) error {
			if runnable, ok := n.(RunnableNode); ok {
				desiredTasks[runnable.NodeID()] = desiredTask{
					rank:     rank,
					groupID:  groupID,
					runnable: runnable,
				}
				rank++
			}
			return nil
		})
	}

	return desiredTasks
}

func startOrder(tasks []*task) []*task {
	slices.SortFunc(tasks, func(a, b *task) int {
		return a.rank - b.rank
	})
	return tasks
}

func stopOrder(tasks []*task) []*task {
	slices.SortFunc(tasks, func(a, b *task) int {
		return b.rank - a.rank
	})
	return tasks
}
