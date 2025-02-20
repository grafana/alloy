//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/elastic/go-freelru"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/reporter"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"

	"github.com/oklog/run"

	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/helpers"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/sd"
	"go.opentelemetry.io/ebpf-profiler/vc"
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
	cgroups, err := freelru.NewSynced[libpf.PID, string](args.ContainerIDCacheSize,
		func(pid libpf.PID) uint32 { return uint32(pid) })

	discovery, err := sd.NewTargetFinder(opts.Logger, cgroups, targetsOptionFromArguments(args))
	if err != nil {
		return nil, fmt.Errorf("ebpf target finder create: %w", err)
	}
	ms := newMetrics(opts.Registerer)

	appendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)

	cfg, err := createConfigFromArguments(args)
	if err != nil {
		return nil, err
	}

	tf := irsymcache.NewTableFactory(cfg.PyroscopeSymbolizerTableGSYM)
	nfs, err := irsymcache.NewFSCache(opts.Logger, tf, irsymcache.Options{
		Enabled: cfg.PyroscopeSymbolizeNativeFrames,
		Size:    cfg.PyroscopeSymbCacheSizeBytes,
		Path:    cfg.PyroscopeSymbCachePath,
	})
	if err != nil {
		return nil, err
	}
	cfg.NativeFrameSymbolizer = nfs
	if cfg.PyroscopeDynamicProfilingPolicy {
		cfg.Policy = &dynamicprofiling.ServiceDiscoveryTargetsOnlyPolicy{Discovery: discovery}
	} else {
		cfg.Policy = dynamicprofiling.AlwaysOnPolicy{}
	}

	cfg.Reporter, err = reporter.New(opts.Logger, cgroups, cfg, discovery, nfs, &pprofConsumer{fanout: appendable, logger: opts.Logger})
	if err != nil {
		return nil, err
	}
	res := &Component{
		cfg:          cfg,
		options:      opts,
		metrics:      ms,
		appendable:   appendable,
		args:         args,
		targetFinder: discovery,
		argsUpdate:   make(chan Arguments),
	}
	return res, nil
}

// NewDefaultArguments create the default settings for a scrape job.
func NewDefaultArguments() Arguments {
	return Arguments{
		CollectInterval:      15 * time.Second,
		SampleRate:           19,
		ContainerIDCacheSize: 1024,
		CollectUserProfile:   true,
		CollectKernelProfile: true,
		Demangle:             "none",
		PythonEnabled:        true,
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

	debugInfo     DebugInfo
	debugInfoLock sync.Mutex
	metrics       *metrics
	cfg           *controller.Config
}

func (c *Component) Run(ctx context.Context) error {
	c.options.Logger.Log(fmt.Sprintf("Starting OTEL profiling agent %s (revision %s, build timestamp %s)",
		vc.Version(), vc.Revision(), vc.BuildTimestamp())) //todo extract this info from mod if possible?
	ctlr := controller.New(c.cfg)
	err := ctlr.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start agent controller: %v", err)
	}
	defer ctlr.Shutdown()

	var g run.Group
	g.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case newArgs := <-c.argsUpdate:
				c.args = newArgs
				c.targetFinder.Update(targetsOptionFromArguments(c.args))
				c.appendable.UpdateChildren(newArgs.ForwardTo)
			}
		}
	}, func(error) {

	})
	return g.Run()
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.argsUpdate <- newArgs
	return nil
}

func (c *Component) DebugInfo() interface{} {
	c.debugInfoLock.Lock()
	defer c.debugInfoLock.Unlock()
	return c.debugInfo
}

type DebugInfo struct {
}

func (c *Component) updateDebugInfo() {
	c.debugInfoLock.Lock()
	defer c.debugInfoLock.Unlock()

	c.debugInfo = DebugInfo{}
}

func targetsOptionFromArguments(args Arguments) sd.TargetsOptions {
	targets := make([]sd.DiscoveryTarget, 0, len(args.Targets))
	for _, t := range args.Targets {
		targets = append(targets, sd.DiscoveryTarget(t))
	}
	return sd.TargetsOptions{
		Targets:     targets,
		TargetsOnly: true,
	}
}

func createConfigFromArguments(args Arguments) (*controller.Config, error) {
	cfgProtoType, err := controller.ParseArgs()
	if err != nil {
		return nil, err
	}

	if err = cfgProtoType.Validate(); err != nil {
		return nil, err
	}

	// hostname and sourceIP will be populated from the root namespace.
	hostname, sourceIP, err := helpers.GetHostnameAndSourceIP(cfgProtoType.CollAgentAddr)
	if err != nil {
		return nil, err
	}
	cfgProtoType.HostName, cfgProtoType.IPAddress = hostname, sourceIP

	cfg := new(controller.Config)
	*cfg = *cfgProtoType
	cfg.ReporterInterval = args.CollectInterval
	cfg.SamplesPerSecond = args.SampleRate
	cfg.Tracers = "perl,php,hotspot,ruby,v8,dotnet"
	if args.PythonEnabled { // todo create flags for other interpreters
		cfg.Tracers += ",python"
	}
	return cfg, nil

}

type pprofConsumer struct {
	fanout *pyroscope.Fanout
	logger log.Logger
}

func (p2 *pprofConsumer) Next(p []reporter.PPROF) {
	for _, pprof := range p {
		appender := p2.fanout.Appender()
		err := appender.Append(context.Background(), pprof.Labels, []*pyroscope.RawSample{{RawProfile: pprof.Raw}})
		if err != nil {
			level.Error(p2.logger).Log("msg", "pprof write", "err", err)
		}
	}
}
