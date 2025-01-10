//go:build (linux || darwin) && (amd64 || arm64)

package java

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
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

type debugInfo struct {
	ProfiledTargets []*debugInfoProfiledTarget `alloy:"profiled_targets,block"`
}

type debugInfoBytesPerType struct {
	Type  string `alloy:"type,attr"`
	Bytes int64  `alloy:"bytes,attr"`
}

type debugInfoProfiledTarget struct {
	TotalBytes              int64            `alloy:"total_bytes,attr,optional"`
	TotalSamples            int64            `alloy:"total_samples,attr,optional"`
	LastProfiled            time.Time        `alloy:"last_profiled,attr,optional"`
	LastError               time.Time        `alloy:"last_error,attr,optional"`
	LastProfileBytesPerType map[string]int64 `alloy:"last_profile_bytes_per_type,attr,optional"`
	ErrorMsg                string           `alloy:"error_msg,attr,optional"`
	PID                     int              `alloy:"pid,attr"`
	Target                  discovery.Target `alloy:"target,attr"`
}

var (
	_ component.DebugComponent = (*javaComponent)(nil)
	_ component.Component      = (*javaComponent)(nil)
)

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

func (j *javaComponent) DebugInfo() interface{} {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	var di debugInfo
	di.ProfiledTargets = make([]*debugInfoProfiledTarget, 0, len(j.pid2process))
	for _, proc := range j.pid2process {
		di.ProfiledTargets = append(di.ProfiledTargets, proc.debugInfo())
	}
	// sort by pid
	sort.Slice(di.ProfiledTargets, func(i, j int) bool {
		return di.ProfiledTargets[i].PID < di.ProfiledTargets[j].PID
	})
	return &di
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
