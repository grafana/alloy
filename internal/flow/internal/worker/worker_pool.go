package worker

import (
	"fmt"
	"runtime"
	"sync"
)

type Pool interface {
	// Stop stops the worker pool. It does not wait to drain any internal queues, but it does wait for the currently
	// running tasks to complete. It must only be called once.
	Stop()
	// SubmitWithKey submits a function to be executed by the worker pool, ensuring that:
	//   * Only one job with given key can be waiting to be executed at the time. This is desired if we don't want to
	//     run the same task multiple times, e.g. if it's a component update that we only need to run once.
	//   * Only one job with given key can be running at the time. This is desired when we don't want to duplicate work,
	//     and we want to protect the pool from a slow task hogging all the workers.
	//
	// Note that it is possible to have two tasks with the same key in the pool at the same time: one waiting to be
	// executed and another one running. This ensures that a request to re-run a task with the same key is not lost.
	//
	// Adding a job with a key that is already queued is a no-op (even if the submitted function is different).
	// Error is returned if the pool is unable to accept extra work - the caller can decide how to handle this situation.
	SubmitWithKey(string, func()) error
	// QueueSize returns the number of tasks currently queued or running.
	QueueSize() int
}

// fixedWorkerPool is a Pool that distributes work across a fixed number of workers. It uses workQueue to ensure
// that SubmitWithKey guarantees are met.
type fixedWorkerPool struct {
	workersCount int
	workQueue    *workQueue
	quit         chan struct{}
	allStopped   sync.WaitGroup
}

var _ Pool = (*fixedWorkerPool)(nil)

func NewDefaultWorkerPool() Pool {
	return NewFixedWorkerPool(runtime.NumCPU(), 1024)
}

// NewFixedWorkerPool creates a new Pool with the given number of workers and given max queue size.
// The max queue size is the maximum number of tasks that can be queued OR running at the same time.
// The tasks can run on a random worker, but workQueue ensures only one task with given key is running at a time.
// The pool is automatically started and ready to accept work. To prevent resource leak, Stop() must be called when the
// pool is no longer needed.
func NewFixedWorkerPool(workersCount int, maxQueueSize int) Pool {
	if workersCount <= 0 {
		panic(fmt.Sprintf("workersCount must be positive, got %d", workersCount))
	}
	pool := &fixedWorkerPool{
		workersCount: workersCount,
		workQueue:    newWorkQueue(maxQueueSize),
		quit:         make(chan struct{}),
	}
	pool.start()
	return pool
}

func (w *fixedWorkerPool) SubmitWithKey(key string, f func()) error {
	_, err := w.workQueue.tryEnqueue(key, f)
	return err
}

// QueueSize returns the number of tasks in the queue - waiting or currently running.
func (w *fixedWorkerPool) QueueSize() int {
	return w.workQueue.queueSize()
}

func (w *fixedWorkerPool) Stop() {
	close(w.quit)
	w.allStopped.Wait()
}

func (w *fixedWorkerPool) start() {
	for i := 0; i < w.workersCount; i++ {
		w.allStopped.Add(1)
		go func() {
			defer w.allStopped.Done()
			for {
				select {
				case <-w.quit:
					return
				case f := <-w.workQueue.tasksToRun:
					f()
				}
			}
		}()
	}
}

type workQueue struct {
	maxSize    int
	tasksToRun chan func()

	lock         sync.Mutex
	waitingOrder []string
	waiting      map[string]func()
	running      map[string]struct{}
}

func newWorkQueue(maxSize int) *workQueue {
	return &workQueue{
		maxSize:    maxSize,
		tasksToRun: make(chan func(), maxSize),
		waiting:    make(map[string]func()),
		running:    make(map[string]struct{}),
	}
}

func (w *workQueue) tryEnqueue(key string, f func()) (bool, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	// Don't enqueue if same task already waiting
	if _, exists := w.waiting[key]; exists {
		return false, nil
	}

	// Don't exceed queue size
	queueSize := len(w.waitingOrder) + len(w.running)
	if queueSize >= w.maxSize {
		return false, fmt.Errorf("worker queue is full")
	}

	// Else enqueue
	w.waitingOrder = append(w.waitingOrder, key)
	w.waiting[key] = f

	// A task may have become runnable now, emit it
	w.emitNextTask()

	return true, nil
}

func (w *workQueue) taskDone(key string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	delete(w.running, key)
	// A task may have become runnable now, emit it
	w.emitNextTask()
}

// emitNextTask emits the next eligible task to be run if there is one. It must be called whenever the queue state
// changes (e.g. a task is added or a task finishes). The lock must be held when calling this function.
func (w *workQueue) emitNextTask() {
	var (
		task  func()
		key   string
		index int
		found = false
	)

	// Find the first key in waitingOrder that is not yet running
	for i, k := range w.waitingOrder {
		if _, alreadyRunning := w.running[k]; !alreadyRunning {
			found, key, index = true, k, i
			break
		}
	}

	// Return if we didn't find any task ready to run
	if !found {
		return
	}

	// Remove the task from waiting and add it to running set.
	// NOTE: Even though we remove an element from the middle of a collection, we use a slice instead of a linked list.
	// This code is NOT identified as a performance hot spot and given that in large agents we observe max number of
	// tasks queued to be ~10, the slice is actually faster because it does not allocate memory. See BenchmarkQueue.
	w.waitingOrder = append(w.waitingOrder[:index], w.waitingOrder[index+1:]...)
	task = w.waiting[key]
	delete(w.waiting, key)
	w.running[key] = struct{}{}

	// Wrap the actual task to make sure we mark it as done when it finishes
	wrapped := func() {
		defer w.taskDone(key)
		task()
	}

	// Emit the task to be run. There will always be space in this buffered channel, because we limit queue size.
	w.tasksToRun <- wrapped
}

func (w *workQueue) queueSize() int {
	w.lock.Lock()
	defer w.lock.Unlock()
	return len(w.waitingOrder) + len(w.running)
}
