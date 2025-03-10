//go:build linux && (arm64 || amd64) && pyroscope_ebpf

package ebpf

import (
	"context"
	"fmt"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter"
	"strings"
	"time"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/reporter/samples"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"

	"github.com/oklog/run"

	sd "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
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
	cfg, err := createConfigFromArguments(args)
	if err != nil {
		return nil, err
	}
	dynamicProfilingPolicy := cfg.PyroscopeDynamicProfilingPolicy
	discovery, err := sd.NewTargetProducer(cgroups, targetsOptions(dynamicProfilingPolicy, args))
	if err != nil {
		return nil, fmt.Errorf("ebpf target finder create: %w", err)
	}
	ms := newMetrics(opts.Registerer)

	appendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)

	var nfs samples.NativeSymbolResolver
	if cfg.SymbolizeNativeFrames {
		tf := irsymcache.NewTableFactory()
		nfs, err = irsymcache.NewFSCache(tf, irsymcache.Options{
			Size: cfg.SymbCacheSizeBytes,
			Path: cfg.SymbCachePath,
		})
		if err != nil {
			return nil, err
		}
	}
	cfg.FileObserver = nfs

	if dynamicProfilingPolicy {
		cfg.Policy = &dynamicprofiling.ServiceDiscoveryTargetsOnlyPolicy{Discovery: discovery}
	} else {
		cfg.Policy = dynamicprofiling.AlwaysOnPolicy{}
	}

	res := &Component{
		cfg:                    cfg,
		options:                opts,
		metrics:                ms,
		appendable:             appendable,
		args:                   args,
		targetFinder:           discovery,
		dynamicProfilingPolicy: dynamicProfilingPolicy,
		argsUpdate:             make(chan Arguments),
	}

	cfg.Reporter, err = reporter.New(opts.Logger, cgroups, cfg, discovery, nfs, res)
	if err != nil {
		return nil, err
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
	options                component.Options
	args                   Arguments
	dynamicProfilingPolicy bool
	argsUpdate             chan Arguments
	appendable             *pyroscope.Fanout
	targetFinder           sd.TargetProducer

	metrics *metrics
	cfg     *controller.Config
}

func (c *Component) Run(ctx context.Context) error {
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
				c.targetFinder.Update(targetsOptions(c.dynamicProfilingPolicy, c.args))
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

func targetsOptions(dynamicProfilingPolicy bool, args Arguments) sd.TargetsOptions {
	targets := make([]sd.DiscoveredTarget, 0, len(args.Targets))
	for _, t := range args.Targets {
		targets = append(targets, t.AsMap()) // todo optimize AsMap
	}
	return sd.TargetsOptions{
		Targets:     targets,
		TargetsOnly: dynamicProfilingPolicy,
		DefaultTarget: sd.DiscoveredTarget{
			"service_name": "unspecified",
		},
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

	cfg := new(controller.Config)
	*cfg = *cfgProtoType
	cfg.ReporterInterval = args.CollectInterval
	cfg.SamplesPerSecond = args.SampleRate
	cfg.Tracers = tracersFromArgs(args)
	return cfg, nil
}

func tracersFromArgs(args Arguments) string {
	var tracers []string
	if args.PythonEnabled {
		tracers = append(tracers, "python")
	}
	if args.PerlEnabled {
		tracers = append(tracers, "perl")
	}
	if args.PHPEnabled {
		tracers = append(tracers, "php")
	}
	if args.HotspotEnabled {
		tracers = append(tracers, "hotspot")
	}
	if args.V8Enabled {
		tracers = append(tracers, "v8")
	}
	if args.RubyEnabled {
		tracers = append(tracers, "ruby")
	}
	if args.DotNetEnabled {
		tracers = append(tracers, "dotnet")
	}
	return strings.Join(tracers, ",")
}

func (c *Component) ConsumePprofProfiles(pprofs []reporter.PPROF) {
	for _, pprof := range pprofs {
		appender := c.appendable.Appender()
		err := appender.Append(context.Background(), pprof.Labels, []*pyroscope.RawSample{{RawProfile: pprof.Raw}})
		if err != nil {
			_ = level.Error(c.options.Logger).Log("msg", "pprof write", "err", err)
		}
	}
}
