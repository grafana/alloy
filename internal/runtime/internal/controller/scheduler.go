package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"

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
	ctx                  context.Context
	cancel               context.CancelFunc
	running              sync.WaitGroup
	logger               log.Logger
	taskShutdownDeadline time.Duration

	tasksMut      sync.Mutex
	tasks         map[string]*task
	stoppingOrder []string
}

// NewScheduler creates a new Scheduler. Call Synchronize to manage the set of
// components which are running.
//
// Call Close to stop the Scheduler and all running components.
func NewScheduler(logger log.Logger, taskShutdownDeadline time.Duration) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:                  ctx,
		cancel:               cancel,
		logger:               logger,
		taskShutdownDeadline: taskShutdownDeadline,

		tasks: make(map[string]*task),
	}
}

// Synchronize adjusts the set of running components based on the provided
// RunnableNodes in the following way,
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
// Existing components will be restarted if they stopped since the previous
// call to Synchronize.
//
// Synchronize is not goroutine safe and should not be called concurrently.
func (s *Scheduler) Synchronize(rr []RunnableNode) error {
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("scheduler is closed")
	default:
	}

	var (
		taskLen          = len(rr)
		newRunnables     = make(map[string]RunnableNode, taskLen)
		toStop           = make([]*task, 0, taskLen)
		newStoppingOrder = make([]string, taskLen)
	)

	for i, r := range rr {
		id := r.NodeID()
		newRunnables[id] = r
		// We stop tasks in reversed order from how they are scheduled.
		newStoppingOrder[taskLen-1-i] = id
	}

	s.tasksMut.Lock()
	for _, id := range s.stoppingOrder {
		if _, keep := newRunnables[id]; !keep {
			if task, ok := s.tasks[id]; ok {
				toStop = append(toStop, task)
			}
		}
	}
	s.tasksMut.Unlock()

	doneStopping := make(chan struct{})
	go func() {
		for _, t := range toStop {
			// NOTE: we cannot hold lock when calling Stop because onDone will mutate running tasks.
			t.Stop()
		}
		close(doneStopping)
	}()

	stoppingTimedOut := false
	select {
	case <-doneStopping:
		// All tasks stopped successfully within timeout.
	case <-time.After(TaskShutdownWarningTimeout):
		level.Warn(s.logger).Log("msg", "Some tasks are taking longer than expected to shutdown, proceeding with new tasks")
		stoppingTimedOut = true
	}

	s.tasksMut.Lock()
	s.stoppingOrder = newStoppingOrder
	// Launch new runnables that have appeared.
	for id, r := range newRunnables {
		if _, exist := s.tasks[id]; exist {
			continue
		}

		level.Debug(s.logger).Log("msg", "Starting task", "id", id)
		var (
			nodeID      = id
			newRunnable = r
		)

		s.running.Add(1)
		s.tasks[nodeID] = newTask(taskOptions{
			runnable: newRunnable,
			onDone: func(err error) {
				defer s.running.Done()
				if err != nil {
					level.Error(s.logger).Log("msg", "node exited with error", "node", nodeID, "err", err)
				} else {
					level.Info(s.logger).Log("msg", "node exited without error", "node", nodeID)
				}

				s.tasksMut.Lock()
				defer s.tasksMut.Unlock()
				delete(s.tasks, nodeID)
			},
			logger:               log.With(s.logger, "taskID", nodeID),
			taskShutdownDeadline: s.taskShutdownDeadline,
		})
	}
	s.tasksMut.Unlock()

	// If we timed out, wait for stopping tasks to fully exit before returning.
	// Tasks shutting down cannot fully complete their shutdown until the taskMut
	// lock is released.
	if stoppingTimedOut {
		<-doneStopping
	}

	return nil
}

// Stop stops the Scheduler and returns after all running goroutines have
// exited.
func (s *Scheduler) Stop() {
	s.cancel()

	s.tasksMut.Lock()
	toStop := make([]*task, 0, len(s.tasks))
	for _, id := range s.stoppingOrder {
		if task, ok := s.tasks[id]; ok {
			toStop = append(toStop, task)
		}
	}
	s.tasksMut.Unlock()

	for _, t := range toStop {
		t.Stop()
	}

	s.running.Wait()
}

// task is a scheduled runnable.
type task struct {
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
func newTask(opts taskOptions) *task {
	ctx, cancel := context.WithCancel(context.Background())

	t := &task{
		ctx:    ctx,
		cancel: cancel,
		exited: make(chan struct{}),
		opts:   opts,
	}

	go func() {
		err := opts.runnable.Run(t.ctx)
		close(t.exited)
		t.doneOnce.Do(func() {
			t.opts.onDone(err)
		})
	}()
	return t
}

func (t *task) Stop() {
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
