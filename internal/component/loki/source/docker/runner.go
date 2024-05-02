package docker

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).

import (
	"context"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	dt "github.com/grafana/alloy/internal/component/loki/source/docker/internal/dockertarget"
	"github.com/grafana/alloy/internal/runner"
)

// A manager manages a set of running tailers.
type manager struct {
	log log.Logger

	mut   sync.Mutex
	opts  *options
	tasks []*tailerTask

	runner *runner.Runner[*tailerTask]
}

// newManager returns a new Manager which manages a set of running tailers.
// Options must not be modified after passing it to a Manager.
//
// If newManager is called with a nil set of options, no targets will be
// scheduled for running until UpdateOptions is called.
func newManager(l log.Logger, opts *options) *manager {
	return &manager{
		log:  l,
		opts: opts,
		runner: runner.New(func(t *tailerTask) runner.Worker {
			return newTailer(l, t)
		}),
	}
}

// options passed to all tailers.
type options struct {
	// client to use to request logs from Docker.
	client client.APIClient

	// handler to send discovered logs to.
	handler loki.EntryHandler

	// positions interface so tailers can save/restore offsets in log files.
	positions positions.Positions

	// taskRestartInterval to restart task that has stopped running.
	taskRestartInterval time.Duration
}

// tailerTask is the payload used to create tailers. It implements runner.Task.
type tailerTask struct {
	options *options
	target  *dt.Target
}

var _ runner.Task = (*tailerTask)(nil)

func (tt *tailerTask) Hash() uint64 { return tt.target.Hash() }

func (tt *tailerTask) Equals(other runner.Task) bool {
	otherTask := other.(*tailerTask)

	// Quick path: pointers are exactly the same.
	if tt == otherTask {
		return true
	}

	// Slow path: check individual fields which are part of the task.
	return tt.options == otherTask.options &&
		tt.target.LabelsStr() == otherTask.target.LabelsStr()
}

// A tailer tails the logs of a docker container. It is created by a [Manager].
type tailer struct {
	log    log.Logger
	opts   *options
	target *dt.Target
}

// newTailer returns a new tailer which tails logs from the target specified by
// the task.
func newTailer(l log.Logger, task *tailerTask) *tailer {
	return &tailer{
		log:    log.WithPrefix(l, "target", task.target.Name()),
		opts:   task.options,
		target: task.target,
	}
}

func (t *tailer) Run(ctx context.Context) {
	ticker := time.NewTicker(t.opts.taskRestartInterval)
	tickerC := ticker.C

	for {
		select {
		case <-tickerC:
			res, err := t.opts.client.ContainerInspect(ctx, t.target.Name())
			if err != nil {
				level.Error(t.log).Log("msg", "error inspecting Docker container", "id", t.target.Name(), "error", err)
				continue
			}
			if res.State.Running {
				t.target.StartIfNotRunning()
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// syncTargets synchronizes the set of running tailers to the set specified by
// targets.
func (m *manager) syncTargets(ctx context.Context, targets []*dt.Target) error {
	m.mut.Lock()
	defer m.mut.Unlock()

	// Convert targets into tasks to give to the runner.
	tasks := make([]*tailerTask, 0, len(targets))
	for _, target := range targets {
		tasks = append(tasks, &tailerTask{
			options: m.opts,
			target:  target,
		})
	}

	// Sync our tasks to the runner. If the Manager doesn't have any options,
	// the runner will be cleared of tasks until UpdateOptions is called with a
	// non-nil set of options.
	switch m.opts {
	default:
		if err := m.runner.ApplyTasks(ctx, tasks); err != nil {
			return err
		}
	case nil:
		if err := m.runner.ApplyTasks(ctx, nil); err != nil {
			return err
		}
	}

	// Delete positions for targets which have gone away.
	newEntries := make(map[positions.Entry]struct{}, len(targets))
	for _, target := range targets {
		newEntries[entryForTarget(target)] = struct{}{}
	}

	for _, task := range m.tasks {
		ent := entryForTarget(task.target)

		// The task from the last call to SyncTargets is no longer in newEntries;
		// remove it from the positions file. We do this _after_ calling ApplyTasks
		// to ensure that the old tailers have shut down, otherwise the tailer
		// might write its position again during shutdown after we removed it.
		if _, found := newEntries[ent]; !found {
			level.Info(m.log).Log("msg", "removing entry from positions file", "path", ent.Path, "labels", ent.Labels)
			m.opts.positions.Remove(ent.Path, ent.Labels)
		}
	}

	m.tasks = tasks
	return nil
}

func entryForTarget(t *dt.Target) positions.Entry {
	// The positions entry is keyed by container_id; the path is fed into
	// positions.CursorKey to treat it as a "cursor"; otherwise
	// positions.Positions will try to read the path as a file and delete the
	// entry when it can't find it.
	return positions.Entry{
		Path:   positions.CursorKey(t.Name()),
		Labels: t.LabelsStr(),
	}
}

// updateOptions updates the Options shared with all Tailers. All Tailers will
// be updated with the new set of Options. Options should not be modified after
// passing to updateOptions.
//
// If newOptions is nil, all tasks will be cleared until updateOptions is
// called again with a non-nil set of options.
func (m *manager) updateOptions(ctx context.Context, newOptions *options) error {
	m.mut.Lock()
	defer m.mut.Unlock()

	// Iterate through the previous set of tasks and create a new task with the
	// new set of options.
	tasks := make([]*tailerTask, 0, len(m.tasks))
	for _, oldTask := range m.tasks {
		tasks = append(tasks, &tailerTask{
			options: newOptions,
			target:  oldTask.target,
		})
	}

	switch newOptions {
	case nil:
		if err := m.runner.ApplyTasks(ctx, nil); err != nil {
			return err
		}
	default:
		if err := m.runner.ApplyTasks(ctx, tasks); err != nil {
			return err
		}
	}

	m.opts = newOptions
	m.tasks = tasks
	return nil
}

// targets returns the set of targets which are actively being tailed. targets
// for tailers which have terminated are not included. The returned set of
// targets are deduplicated.
func (m *manager) targets() []*dt.Target {
	tasks := m.runner.Tasks()

	targets := make([]*dt.Target, 0, len(tasks))
	for _, task := range tasks {
		targets = append(targets, task.target)
	}
	return targets
}

// stop stops the manager and all running Tailers. It blocks until all Tailers
// have exited.
func (m *manager) stop() {
	m.runner.Stop()
}
