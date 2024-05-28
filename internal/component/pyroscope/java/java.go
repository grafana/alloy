//go:build linux && (amd64 || arm64)

package java

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/java/asprof"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	labelProcessID = "__process_pid__"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.java",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			if os.Getuid() != 0 {
				return nil, fmt.Errorf("java profiler: must be run as root")
			}
			a := args.(Arguments)
			var profiler = asprof.NewProfiler(a.TmpDir, asprof.EmbeddedArchive)
			err := profiler.ExtractDistributions()
			if err != nil {
				return nil, fmt.Errorf("extract async profiler: %w", err)
			}

			forwardTo := pyroscope.NewFanout(a.ForwardTo, opts.ID, opts.Registerer)
			c := &javaComponent{
				opts:        opts,
				args:        a,
				forwardTo:   forwardTo,
				profiler:    profiler,
				pid2process: make(map[int]*profilingLoop),
			}
			c.updateTargets(a)
			return c, nil
		},
	})
}

type javaComponent struct {
	opts      component.Options
	args      Arguments
	forwardTo *pyroscope.Fanout

	mutex       sync.Mutex
	pid2process map[int]*profilingLoop
	profiler    *asprof.Profiler
}

func (j *javaComponent) Run(ctx context.Context) error {
	defer func() {
		j.stop()
	}()
	<-ctx.Done()
	return nil
}

func (j *javaComponent) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	j.forwardTo.UpdateChildren(newArgs.ForwardTo)
	j.updateTargets(newArgs)
	return nil
}

func (j *javaComponent) updateTargets(args Arguments) {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	j.args = args

	active := make(map[int]struct{})
	for _, target := range args.Targets {
		pid, err := strconv.Atoi(target[labelProcessID])
		_ = level.Debug(j.opts.Logger).Log("msg", "active target",
			"target", fmt.Sprintf("%+v", target),
			"pid", pid)
		if err != nil {
			_ = level.Error(j.opts.Logger).Log("msg", "invalid target", "target", fmt.Sprintf("%v", target), "err", err)
			continue
		}
		proc := j.pid2process[pid]
		if proc == nil {
			proc = newProfilingLoop(pid, target, j.opts.Logger, j.profiler, j.forwardTo, j.args.ProfilingConfig)
			_ = level.Debug(j.opts.Logger).Log("msg", "new process", "target", fmt.Sprintf("%+v", target))
			j.pid2process[pid] = proc
		} else {
			proc.update(target, j.args.ProfilingConfig)
		}
		active[pid] = struct{}{}
	}
	for pid := range j.pid2process {
		if _, ok := active[pid]; ok {
			continue
		}
		_ = level.Debug(j.opts.Logger).Log("msg", "inactive target", "pid", pid)
		_ = j.pid2process[pid].Close()
		delete(j.pid2process, pid)
	}
}

func (j *javaComponent) stop() {
	_ = level.Debug(j.opts.Logger).Log("msg", "stopping")
	j.mutex.Lock()
	defer j.mutex.Unlock()
	for _, proc := range j.pid2process {
		proc.Close()
		_ = level.Debug(j.opts.Logger).Log("msg", "stopped", "pid", proc.pid)
		delete(j.pid2process, proc.pid)
	}
}
