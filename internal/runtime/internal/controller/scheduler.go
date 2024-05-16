package controller

import (
	"context"
	"fmt"
	"sync"
)

// RunnableNode is any BlockNode which can also be run.
type RunnableNode interface {
	BlockNode
	Run(ctx context.Context) error
}

// Scheduler runs components.
type Scheduler struct {
	ctx     context.Context
	cancel  context.CancelFunc
	running sync.WaitGroup

	tasksMut sync.Mutex
	tasks    map[string]*task
}

// NewScheduler creates a new Scheduler. Call Synchronize to manage the set of
// components which are running.
//
// Call Close to stop the Scheduler and all running components.
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:    ctx,
		cancel: cancel,

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
	defer s.tasksMut.Unlock()

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
			Context:  s.ctx,
			Runnable: newRunnable,
			OnDone: func() {
				defer s.running.Done()

				s.tasksMut.Lock()
				defer s.tasksMut.Unlock()
				delete(s.tasks, nodeID)
			},
		}

		s.running.Add(1)
		s.tasks[nodeID] = newTask(opts)
	}

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
	ctx    context.Context
	cancel context.CancelFunc
	exited chan struct{}
}

type taskOptions struct {
	Context  context.Context
	Runnable RunnableNode
	OnDone   func()
}

// newTask creates and starts a new task.
func newTask(opts taskOptions) *task {
	ctx, cancel := context.WithCancel(opts.Context)

	t := &task{
		ctx:    ctx,
		cancel: cancel,
		exited: make(chan struct{}),
	}

	go func() {
		defer opts.OnDone()
		defer close(t.exited)
		_ = opts.Runnable.Run(t.ctx)
	}()
	return t
}

func (t *task) Stop() {
	t.cancel()
	<-t.exited
}
