//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	ebpfspy "github.com/grafana/pyroscope/ebpf"
	demangle2 "github.com/grafana/pyroscope/ebpf/cpp/demangle"
	"github.com/grafana/pyroscope/ebpf/pprof"
	"github.com/grafana/pyroscope/ebpf/sd"
	"github.com/grafana/pyroscope/ebpf/symtab"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.ebpf",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			arguments := args.(Arguments)
			return New(opts, arguments)
		},
	})
}

func New(opts component.Options, args Arguments) (component.Component, error) {
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/"), opts.Logger, targetsOptionFromArgs(args))
	if err != nil {
		return nil, fmt.Errorf("ebpf target finder create: %w", err)
	}
	ms := newMetrics(opts.Registerer)

	session, err := ebpfspy.NewSession(
		opts.Logger,
		targetFinder,
		convertSessionOptions(args, ms),
	)
	if err != nil {
		return nil, fmt.Errorf("ebpf session create: %w", err)
	}

	alloyAppendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)

	res := &Component{
		options:      opts,
		metrics:      ms,
		appendable:   alloyAppendable,
		args:         args,
		targetFinder: targetFinder,
		session:      session,
		argsUpdate:   make(chan Arguments, 4),
	}
	res.metrics.targetsActive.Set(float64(len(res.targetFinder.DebugInfo())))
	return res, nil
}

var DefaultArguments = NewDefaultArguments()

// NewDefaultArguments create the default settings for a scrape job.
func NewDefaultArguments() Arguments {
	return Arguments{
		CollectInterval:      15 * time.Second,
		SampleRate:           97,
		PidCacheSize:         32,
		ContainerIDCacheSize: 1024,
		BuildIDCacheSize:     64,
		SameFileCacheSize:    8,
		CacheRounds:          3,
		CollectUserProfile:   true,
		CollectKernelProfile: true,
		Demangle:             "none",
		PythonEnabled:        true,
		SymbolsMapSize:       2048,
		PIDMapSize:           16384,
	}
}

// SetToDefault implements syntax.Defaulter.
func (arg *Arguments) SetToDefault() {
	*arg = NewDefaultArguments()
}

type Component struct {
	options      component.Options
	args         Arguments
	argsUpdate   chan Arguments
	appendable   *pyroscope.Fanout
	targetFinder sd.TargetFinder
	session      ebpfspy.Session

	debugInfo     DebugInfo
	debugInfoLock sync.Mutex
	metrics       *metrics

	healthMut sync.RWMutex
	health    component.Health
}

func (c *Component) Run(ctx context.Context) error {
	started := false

	collectInterval := c.args.CollectInterval
	t := time.NewTicker(collectInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case newArgs := <-c.argsUpdate:
			// ensure there are no other updates queued. this might happen if the collection takes a very long time
			newArgs = getLatestArgsFromChannel(c.argsUpdate, newArgs)

			// update targets
			c.args = newArgs
			c.session.UpdateTargets(targetsOptionFromArgs(c.args))
			c.metrics.targetsActive.Set(float64(len(c.targetFinder.DebugInfo())))
			err := c.session.Update(convertSessionOptions(c.args, c.metrics))
			if err != nil {
				level.Error(c.options.Logger).Log("msg", "failed to update profiling session", "err", err)
				c.reportUnhealthy(err)
				continue
			}
			c.appendable.UpdateChildren(newArgs.ForwardTo)
			if c.args.CollectInterval != collectInterval {
				t.Reset(c.args.CollectInterval)
				collectInterval = c.args.CollectInterval
			}
		case <-t.C:
			if !started {
				err := c.session.Start()
				if err != nil {
					level.Error(c.options.Logger).Log("msg", "failed to start profiling session", "err", err)
					c.reportUnhealthy(err)
					continue
				}
				defer c.session.Stop()
				started = true
			}

			err := c.collectProfiles()
			if err != nil {
				level.Error(c.options.Logger).Log("msg", "failed to collect profiles", "err", err)
				c.reportUnhealthy(err)
				c.metrics.profilingSessionsFailingTotal.Inc()
				continue
			}
			c.reportHealthy()
			c.updateDebugInfo()
		}
	}
}

func getLatestArgsFromChannel[A any](ch chan A, current A) A {
	for {
		select {
		case x := <-ch:
			current = x
		default:
			return current
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.argsUpdate <- newArgs
	return nil
}

func (c *Component) reportUnhealthy(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     component.HealthTypeUnhealthy,
		Message:    err.Error(),
		UpdateTime: time.Now(),
	}
}

func (c *Component) reportHealthy() {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     component.HealthTypeHealthy,
		UpdateTime: time.Now(),
	}
}

func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

func (c *Component) DebugInfo() interface{} {
	c.debugInfoLock.Lock()
	defer c.debugInfoLock.Unlock()
	return c.debugInfo
}

func (c *Component) collectProfiles() error {
	c.metrics.profilingSessionsTotal.Inc()
	level.Debug(c.options.Logger).Log("msg", "ebpf  collectProfiles")
	args := c.args
	builders := pprof.NewProfileBuilders(pprof.BuildersOptions{
		SampleRate:    int64(args.SampleRate),
		PerPIDProfile: true,
	})
	err := pprof.Collect(builders, c.session)

	if err != nil {
		return fmt.Errorf("ebpf session collectProfiles %w", err)
	}
	level.Debug(c.options.Logger).Log("msg", "ebpf collectProfiles done", "profiles", len(builders.Builders))
	bytesSent := 0
	for _, builder := range builders.Builders {
		serviceName := builder.Labels.Get("service_name")
		c.metrics.pprofsTotal.WithLabelValues(serviceName).Inc()
		c.metrics.pprofSamplesTotal.WithLabelValues(serviceName).Add(float64(len(builder.Profile.Sample)))

		buf := bytes.NewBuffer(nil)
		_, err := builder.Write(buf)
		if err != nil {
			return fmt.Errorf("ebpf profile encode %w", err)
		}
		rawProfile := buf.Bytes()

		appender := c.appendable.Appender()
		bytesSent += len(rawProfile)
		c.metrics.pprofBytesTotal.WithLabelValues(serviceName).Add(float64(len(rawProfile)))

		samples := []*pyroscope.RawSample{{RawProfile: rawProfile}}
		err = appender.Append(context.Background(), builder.Labels, samples)
		if err != nil {
			level.Error(c.options.Logger).Log("msg", "ebpf pprof write", "err", err)
			continue
		}
	}
	level.Debug(c.options.Logger).Log("msg", "ebpf append done", "bytes_sent", bytesSent)
	return nil
}

type DebugInfo struct {
	Targets interface{} `alloy:"targets,attr,optional"`
	Session interface{} `alloy:"session,attr,optional"`
}

func (c *Component) updateDebugInfo() {
	c.debugInfoLock.Lock()
	defer c.debugInfoLock.Unlock()

	c.debugInfo = DebugInfo{
		Targets: c.targetFinder.DebugInfo(),
		Session: c.session.DebugInfo(),
	}
}

func targetsOptionFromArgs(args Arguments) sd.TargetsOptions {
	targets := make([]sd.DiscoveryTarget, 0, len(args.Targets))
	for _, t := range args.Targets {
		targets = append(targets, t.AsMap())
	}
	return sd.TargetsOptions{
		Targets:            targets,
		TargetsOnly:        true,
		ContainerCacheSize: args.ContainerIDCacheSize,
	}
}

func convertSessionOptions(args Arguments, ms *metrics) ebpfspy.SessionOptions {
	return ebpfspy.SessionOptions{
		CollectUser:   args.CollectUserProfile,
		CollectKernel: args.CollectKernelProfile,
		SampleRate:    args.SampleRate,
		PythonEnabled: args.PythonEnabled,
		Metrics:       ms.ebpfMetrics,
		SymbolOptions: symtab.SymbolOptions{
			GoTableFallback: args.GoTableFallback,
			DemangleOptions: demangle2.ConvertDemangleOptions(args.Demangle),
		},
		CacheOptions: symtab.CacheOptions{
			PidCacheOptions: symtab.GCacheOptions{
				Size:       args.PidCacheSize,
				KeepRounds: args.CacheRounds,
			},
			BuildIDCacheOptions: symtab.GCacheOptions{
				Size:       args.BuildIDCacheSize,
				KeepRounds: args.CacheRounds,
			},
			SameFileCacheOptions: symtab.GCacheOptions{
				Size:       args.SameFileCacheSize,
				KeepRounds: args.CacheRounds,
			},
		},
		BPFMapsOptions: ebpfspy.BPFMapsOptions{
			SymbolsMapSize: uint32(args.SymbolsMapSize),
			PIDMapSize:     uint32(args.PIDMapSize),
		},
	}
}
