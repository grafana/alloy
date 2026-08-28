// Package runner provides an API for generic goroutine scheduling. It is
// broken up into three concepts:
//
//  1. Task: a unit of work to perform
//  2. Worker: a goroutine dedicated to doing a specific Task
//  3. Runner: manages the set of Workers, one per unique Task
//
// An example of a Task and Worker pair would be a Task which describes an
// endpoint to poll for health. The Task would then be assigned to a Worker to
// perform the polling.
package runner

import (
	"context"
	"fmt"
	"sync"
)

// A Task is a payload that determines what a Worker should do. For example,
// a Task may be a struct including an address for a Worker to poll.
type Task interface {
	// Hash should return a hash which represents this Task.
	Hash() uint64
	// Equals should determine if two Tasks are equal. It is only called when two
	// Tasks have the same Hash.
	Equals(other Task) bool
}

// A Worker is a goroutine which performs business logic for a Task which is
// assigned to it. Each Worker is responsible for a single Task.
type Worker interface {
	// Run starts a Worker, blocking until the provided ctx is canceled or a
	// fatal error occurs. Run is guaranteed to be called exactly once for any
	// given Worker.
	Run(ctx context.Context)
}

// The Runner manages a set of running Workers based on an active set of tasks.
type Runner[TaskType Task] struct {
	newWorker func(t TaskType) Worker

	ctx    context.Context
	cancel context.CancelFunc

	running sync.WaitGroup
	workers *hashMap
}

// Internal types used to implement the Runner.
type (
	// scheduledWorker is a representation of a running worker.
	scheduledWorker struct {
		Worker Worker // The underlying Worker instance.

		// Function to call to request the worker to stop.
		Cancel context.CancelFunc

		// Exited will close once the worker has exited.
		Exited chan struct{}
	}

	// workerTask represents a tuple of a scheduledWorker with its assigned Task.
	// workerTask implements Task for it to be used in a hashMap; two workerTasks
	// are equal if their underlying Tasks are equal.
	workerTask struct {
		Worker *scheduledWorker
		Task   Task
	}
)

// Hash returns the hash of the Task the scheduledWorker owns.
func (sw *workerTask) Hash() uint64 {
	return sw.Task.Hash()
}

// Equals returns true if the Task owned by this workerTask equals the Task
// owned by another workerTask.
func (sw *workerTask) Equals(other Task) bool {
	return sw.Task.Equals(other.(*workerTask).Task)
}

// New creates a new Runner which manages workers for a given Task type. The
// newWorker function is called whenever a new Task is received that is not
// managed by any existing Worker.
func New[TaskType Task](newWorker func(t TaskType) Worker) *Runner[TaskType] {
	ctx, cancel := context.WithCancel(context.Background())

	return &Runner[TaskType]{
		newWorker: newWorker,

		ctx:    ctx,
		cancel: cancel,

		workers: newHashMap(10),
	}
}

// ApplyTasks updates the Tasks tracked by the Runner to the slice specified
// by t. t should be the entire set of tasks that workers should be operating
// against. ApplyTasks will launch new Workers for new tasks and terminate
// previous Workers for tasks which are no longer found in tt.
//
// ApplyTasks will block until Workers for stale Tasks have terminated. If the
// provided context is canceled, ApplyTasks will still finish synchronizing the
// set of Workers but will not wait for stale Workers to exit.
func (s *Runner[TaskType]) ApplyTasks(ctx context.Context, tt []TaskType) error {
	if s.ctx.Err() != nil {
		return fmt.Errorf("Runner is closed")
	}

	// Create a new hashMap of tasks we intend to run.
	newTasks := newHashMap(len(tt))
	for _, t := range tt {
		newTasks.Add(t)
	}

	// Stop stale workers (i.e., Workers whose tasks are not in newTasks).
	var stopping sync.WaitGroup
	for w := range s.workers.Iterate() {
		if newTasks.Has(w.(*workerTask).Task) {
			// Task still exists.
			continue
		}

		// Stop and remove the task from s.workers.
		stopping.Add(1)
		go func(w *workerTask) {
			defer stopping.Done()
			defer s.workers.Delete(w)
			w.Worker.Cancel()

			select {
			case <-ctx.Done():
			case <-w.Worker.Exited:
			}
		}(w.(*workerTask))
	}

	// Ensure that every defined task in newTasks has a worker associated with
	// it.
	for definedTask := range newTasks.Iterate() {
		// Ignore tasks for workers that are already running.
		//
		// We use a temporary workerTask here where only the task field is used
		// for comparison. This prevents unnecessarily creating a new worker when
		// one isn't needed.
		if s.workers.Has(&workerTask{Task: definedTask}) {
			continue
		}

		workerCtx, workerCancel := context.WithCancel(s.ctx)
		newWorker := &scheduledWorker{
			Worker: s.newWorker(definedTask.(TaskType)),
			Cancel: workerCancel,
			Exited: make(chan struct{}),
		}
		newTask := &workerTask{
			Worker: newWorker,
			Task:   definedTask,
		}

		s.running.Add(1)
		go func() {
			defer s.running.Done()
			defer close(newWorker.Exited)
			newWorker.Worker.Run(workerCtx)
		}()

		_ = s.workers.Add(newTask)
	}

	// Wait for all stopping workers to stop (or until the context to cancel,
	// which will stop the WaitGroup early).
	stopping.Wait()
	return ctx.Err()
}

// Tasks returns the current set of Tasks. Tasks are included even if their
// associated Worker has terminated.
func (s *Runner[TaskType]) Tasks() []TaskType {
	var res []TaskType
	for task := range s.workers.Iterate() {
		workerTask := task.(*workerTask)
		res = append(res, workerTask.Task.(TaskType))
	}
	return res
}

// Workers returns the current set of Workers. Workers are included even if
// they have terminated.
func (s *Runner[TaskType]) Workers() []Worker {
	var res []Worker
	for task := range s.workers.Iterate() {
		workerTask := task.(*workerTask)
		res = append(res, workerTask.Worker.Worker)
	}
	return res
}

// Stop the Scheduler and all running Workers. Close blocks until all running
// Workers exit.
func (s *Runner[TaskType]) Stop() {
	s.cancel()
	s.running.Wait()
}
