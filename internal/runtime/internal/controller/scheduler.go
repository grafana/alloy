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

	tasksMut sync.Mutex
	tasks    map[string]*task
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

// Synchronize synchronizes the running components to those defined by rr.
//
// New RunnableNodes will be launched as new goroutines. RunnableNodes already
// managed by Scheduler will be kept running, while running RunnableNodes that
// are not in rr will be shut down and removed.
//
// Existing components will be restarted if they stopped since the previous
// call to Synchronize.
func (s *Scheduler) Synchronize(rr []RunnableNode) error {
	s.tasksMut.Lock()

	if s.ctx.Err() != nil {
		return fmt.Errorf("Scheduler is closed")
	}

	newRunnables := make(map[string]RunnableNode, len(rr))
	for _, r := range rr {
		newRunnables[r.NodeID()] = r
	}

	// Stop tasks that are not defined in rr.
	var stopping sync.WaitGroup
	for id, t := range s.tasks {
		if _, keep := newRunnables[id]; keep {
			continue
		}

		stopping.Add(1)
		go func(t *task) {
			defer stopping.Done()
			t.Stop()
		}(t)
	}

	// Launch new runnables that have appeared.
	for id, r := range newRunnables {
		if _, exist := s.tasks[id]; exist {
			continue
		}

		var (
			nodeID      = id
			newRunnable = r
		)

		opts := taskOptions{
			context:  s.ctx,
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
		}

		s.running.Add(1)
		s.tasks[nodeID] = newTask(opts)
	}

	// Unlock the tasks mutex so that Stop calls can complete.
	s.tasksMut.Unlock()
	// Wait for all stopping runnables to exit.
	stopping.Wait()
	return nil
}

// Close stops the Scheduler and returns after all running goroutines have
// exited.
func (s *Scheduler) Close() error {
	s.cancel()
	s.running.Wait()
	return nil
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
	context              context.Context
	runnable             RunnableNode
	onDone               func(error)
	logger               log.Logger
	taskShutdownDeadline time.Duration
}

// newTask creates and starts a new task.
func newTask(opts taskOptions) *task {
	ctx, cancel := context.WithCancel(opts.context)

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
