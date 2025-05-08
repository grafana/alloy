package runner_test

import (
	"context"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestRunner_ApplyPayloads(t *testing.T) {
	t.Run("new Workers get scheduled for new tasks", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()

		workerCount := atomic.NewUint64(0)

		r := runner.New(func(t stringTask) runner.Worker {
			return &genericWorker{workerCount: workerCount}
		})
		defer r.Stop()

		var tasks []stringTask

		// Apply the first task and wait for it to run.
		tasks = append(tasks, stringTask("task_a"))
		require.NoError(t, r.ApplyTasks(ctx, tasks))
		requireRunners(t, 1, workerCount)

		// Append a more tasks and wait for it to run.
		tasks = append(tasks, stringTask("task_b"), stringTask("task_c"))
		require.NoError(t, r.ApplyTasks(ctx, tasks))
		requireRunners(t, 3, workerCount)
	})

	t.Run("old Workers get terminated for removed tasks", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()

		workerCount := atomic.NewUint64(0)

		r := runner.New(func(t stringTask) runner.Worker {
			return &genericWorker{workerCount: workerCount}
		})
		defer r.Stop()

		// Apply a set of initial tasks.
		require.NoError(t, r.ApplyTasks(ctx, []stringTask{"task_a", "task_b", "task_c"}))
		requireRunners(t, 3, workerCount)

		// Apply a new set of tasks, removing tasks that were previously defined.
		require.NoError(t, r.ApplyTasks(ctx, []stringTask{"task_b"}))
		requireRunners(t, 1, workerCount)
	})
}

func TestRunner_Stop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	workerCount := atomic.NewUint64(0)

	r := runner.New(func(t stringTask) runner.Worker {
		return &genericWorker{workerCount: workerCount}
	})

	// Apply a set of initial tasks.
	require.NoError(t, r.ApplyTasks(ctx, []stringTask{"task_a", "task_b", "task_c"}))
	requireRunners(t, 3, workerCount)

	// Stop the runner. No tasks should be running afterwards.
	r.Stop()
	requireRunners(t, 0, workerCount)
}

func requireRunners(t *testing.T, expect uint64, actual *atomic.Uint64) {
	util.Eventually(t, func(t require.TestingT) {
		require.Equal(t, expect, actual.Load())
	})
}

type stringTask string

var _ runner.Task = stringTask("")

func (st stringTask) Hash() uint64 {
	return xxhash.Sum64String(string(st))
}

func (st stringTask) Equals(other runner.Task) bool {
	return st == other.(stringTask)
}

type genericWorker struct {
	workerCount *atomic.Uint64
}

var _ runner.Worker = (*genericWorker)(nil)

func (w *genericWorker) Run(ctx context.Context) {
	w.workerCount.Inc()
	defer w.workerCount.Dec()

	<-ctx.Done()
}
