// Package kubetail implements a log file tailer using the Kubernetes API.
package kubetail

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"k8s.io/client-go/kubernetes"
)

// Options passed to all tailers.
type Options struct {
	// Client to use to request logs from Kubernetes.
	Client kubernetes.Interface

	// Handler to send discovered logs to.
	Handler loki.EntryHandler

	// Positions interface so tailers can save/restore offsets in log files.
	Positions positions.Positions

	TailFromEnd bool
}

// A Manager manages a set of running Tailers.
type Manager struct {
	log log.Logger

	mut   sync.Mutex
	opts  *Options
	tasks []*tailerTask

	runner *runner.Runner[*tailerTask]
}

// NewManager returns a new Manager which manages a set of running tailers.
// Options must not be modified after passing it to a Manager.
//
// If NewManager is called with a nil set of options, no targets will be
// scheduled for running until UpdateOptions is called.
func NewManager(l log.Logger, opts *Options) *Manager {
	return &Manager{
		log:  l,
		opts: opts,
		runner: runner.New(func(t *tailerTask) runner.Worker {
			return newTailer(l, t)
		}),
	}
}

// SyncTargets synchronizes the set of running tailers to the set specified by
// targets.
func (m *Manager) SyncTargets(ctx context.Context, targets []*Target) error {
	m.mut.Lock()
	defer m.mut.Unlock()

	// Convert targets into tasks to give to the runner.
	tasks := make([]*tailerTask, 0, len(targets))
	for _, target := range targets {
		tasks = append(tasks, &tailerTask{
			Options: m.opts,
			Target:  target,
		})
	}

	// Sync our tasks to the runner. If the Manager doesn't have any options, the
	// runner will be cleaered of tasks until UpdateOptions is called with a
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
		ent := entryForTarget(task.Target)

		// The task from the last call to SyncTargets is no longer in newEntries;
		// remove it from the positions file. We do this _after_ calling ApplyTasks
		// to ensure that the old tailers have shut down, otherwise the tailer
		// might write its position again during shutdown after we removed it.
		if _, found := newEntries[ent]; !found {
			level.Info(m.log).Log("msg", "removing entry from positions file", "path", ent.Path, "labels", ent.Labels)
			m.opts.Positions.Remove(ent.Path, ent.Labels)
		}
	}

	m.tasks = tasks
	return nil
}

func entryForTarget(t *Target) positions.Entry {
	// The positions entry is keyed by UID to ensure that positions from
	// completely distinct "namespace/name:container" instances don't interfere
	// with each other.
	//
	// While it's still technically possible for two containers to have the same
	// "namespace/name:container" string and UID, it's so wildly unlikely that
	// it's probably not worth handling.
	//
	// The path is fed into positions.CursorKey to treat it as a "cursor";
	// otherwise positions.Positions will try to read the path as a file and
	// delete the entry when it can't find it.
	return positions.Entry{
		Path:   positions.CursorKey(t.String() + ":" + t.UID()),
		Labels: t.Labels().String(),
	}
}

// UpdateOptions updates the Options shared with all Tailers. All Tailers will
// be updated with the new set of Options. Options should not be modified after
// passing to UpdateOptions.
//
// If newOptions is nil, all tasks will be cleared until UpdateOptions is
// called again with a non-nil set of options.
func (m *Manager) UpdateOptions(ctx context.Context, newOptions *Options) error {
	m.mut.Lock()
	defer m.mut.Unlock()

	// Iterate through the previous set of tasks and create a new task with the
	// new set of options.
	tasks := make([]*tailerTask, 0, len(m.tasks))
	for _, oldTask := range m.tasks {
		tasks = append(tasks, &tailerTask{
			Options: newOptions,
			Target:  oldTask.Target,
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

// Targets returns the set of targets which are actively being tailed. Targets
// for tailers which have terminated are not included. The returned set of
// targets are deduplicated.
func (m *Manager) Targets() []*Target {
	tasks := m.runner.Tasks()

	targets := make([]*Target, 0, len(tasks))
	for _, task := range tasks {
		targets = append(targets, task.Target)
	}
	return targets
}

// Stop stops the manager and all running Tailers. It blocks until all Tailers
// have exited.
func (m *Manager) Stop() {
	m.runner.Stop()
}
