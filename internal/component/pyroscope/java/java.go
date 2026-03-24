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

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/java/asprof"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	LabelProcessID = "__process_pid__"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.java",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts.Logger, opts.Registerer, opts.ID, args.(Arguments))
		},
	})
}

func New(logger log.Logger, reg prometheus.Registerer, id string, a Arguments) (*Component, error) {
	if os.Getuid() != 0 {
		return nil, fmt.Errorf("java profiler: must be run as root")
	}
	var (
		dist asprof.Distribution
		err  error
	)
	if a.Dist != "" {
		dist, err = asprof.NewExtractedDistribution(a.Dist)
		if err != nil {
			return nil, fmt.Errorf("invalid asprof dist: %w", err)
		}
		_ = logger.Log("msg", "using extracted asprof dist", "dist", a.Dist)
	} else {
		dist, err = asprof.ExtractDistribution(asprof.EmbeddedArchive, a.TmpDir, asprof.EmbeddedArchive.DistName())
		if err != nil {
			return nil, fmt.Errorf("extract asprof: %w", err)
		}
		_ = logger.Log("msg", "using embedded asprof dist")
	}
	forwardTo := pyroscope.NewFanout(a.ForwardTo, id, reg)
	c := &Component{
		logger:      logger,
		args:        a,
		forwardTo:   forwardTo,
		profiler:    dist,
		pid2process: make(map[int]*profilingLoop),
	}
	c.updateTargets(a)
	return c, nil
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
	_ component.DebugComponent = (*Component)(nil)
	_ component.Component      = (*Component)(nil)
)

type Component struct {
	logger    log.Logger
	args      Arguments
	forwardTo *pyroscope.Fanout

	mutex       sync.Mutex
	pid2process map[int]*profilingLoop
	profiler    asprof.Distribution
}

func (j *Component) Run(ctx context.Context) error {
	defer func() {
		j.stop()
	}()
	<-ctx.Done()
	return nil
}

func (j *Component) DebugInfo() any {
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

func (j *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	j.forwardTo.UpdateChildren(newArgs.ForwardTo)
	j.updateTargets(newArgs)
	return nil
}

func (j *Component) updateTargets(args Arguments) {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	j.args = args

	active := make(map[int]struct{})
	for _, target := range args.Targets {
		pidStr, ok := target.Get(LabelProcessID)
		if !ok {
			_ = level.Error(j.logger).Log("msg", "could not find PID label", "pid", pidStr)
			continue
		}
		pid64, err := strconv.ParseInt(pidStr, 10, 32)
		if err != nil {
			_ = level.Error(j.logger).Log("msg", "could not convert process ID to a 32 bit integer", "pid", pidStr, "err", err)
			continue
		}
		pid := int(pid64)

		_ = level.Debug(j.logger).Log("msg", "active target",
			"target", fmt.Sprintf("%+v", target),
			"pid", pid)
		proc := j.pid2process[pid]
		if proc == nil {
			proc = newProfilingLoop(pid, target, j.logger, j.profiler, j.forwardTo, j.args.ProfilingConfig)
			_ = level.Debug(j.logger).Log("msg", "new process", "target", fmt.Sprintf("%+v", target))
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
		_ = level.Debug(j.logger).Log("msg", "inactive target", "pid", pid)
		_ = j.pid2process[pid].Close()
		delete(j.pid2process, pid)
	}
}

func (j *Component) stop() {
	_ = level.Debug(j.logger).Log("msg", "stopping")
	j.mutex.Lock()
	defer j.mutex.Unlock()
	for _, proc := range j.pid2process {
		proc.Close()
		_ = level.Debug(j.logger).Log("msg", "stopped", "pid", proc.pid)
		delete(j.pid2process, proc.pid)
	}
}
