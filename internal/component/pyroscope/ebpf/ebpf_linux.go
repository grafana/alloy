//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/pyroscope/ebpf/sd"
	"github.com/oklog/run"

	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/helpers"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/times"
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
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/"), opts.Logger, targetsOptionFromArgs(args))
	if err != nil {
		return nil, fmt.Errorf("ebpf target finder create: %w", err)
	}
	ms := newMetrics(opts.Registerer)

	appendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)

	cfg, err := createConfigFromArguments(args)
	if err != nil {
		return nil, err
	}

	cfg.Reporter, err = createOTLPReporter(cfg)
	if err != nil {
		return nil, err
	}
	res := &Component{
		cfg:          cfg,
		options:      opts,
		metrics:      ms,
		appendable:   appendable,
		args:         args,
		targetFinder: targetFinder,
		argsUpdate:   make(chan Arguments),
	}
	res.metrics.targetsActive.Set(float64(len(res.targetFinder.DebugInfo())))
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
		return fmt.Errorf("Failed to start agent controller: %v", err)
	}
	defer ctlr.Shutdown()

	//err := c.session.Start()
	//if err != nil {
	//	return fmt.Errorf("ebpf profiling session start: %w", err)
	//}
	//defer c.session.Stop()

	var g run.Group
	g.Add(func() error {
		collectInterval := c.args.CollectInterval
		t := time.NewTicker(collectInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case newArgs := <-c.argsUpdate:
				c.args = newArgs
				//c.session.UpdateTargets(targetsOptionFromArgs(c.args))
				c.metrics.targetsActive.Set(float64(len(c.targetFinder.DebugInfo())))
				//err := c.session.Update(createConfigFromArguments(c.args, c.metrics))
				//if err != nil {
				//	return nil
				//}
				c.appendable.UpdateChildren(newArgs.ForwardTo)
				if c.args.CollectInterval != collectInterval {
					t.Reset(c.args.CollectInterval)
					collectInterval = c.args.CollectInterval
				}
			case <-t.C:
				err := c.collectProfiles()
				if err != nil {
					c.metrics.profilingSessionsFailingTotal.Inc()
					return err
				}
				c.updateDebugInfo()
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

func (c *Component) collectProfiles() error {
	c.metrics.profilingSessionsTotal.Inc()
	level.Debug(c.options.Logger).Log("msg", "ebpf  collectProfiles")
	//args := c.args
	//builders := pprof.NewProfileBuilders(pprof.BuildersOptions{
	//	SampleRate:    int64(args.SampleRate),
	//	PerPIDProfile: true,
	//})
	//err := pprof.Collect(builders, c.session)
	//
	//if err != nil {
	//	return fmt.Errorf("ebpf session collectProfiles %w", err)
	//}
	//level.Debug(c.options.Logger).Log("msg", "ebpf collectProfiles done", "profiles", len(builders.Builders))
	//bytesSent := 0
	//for _, builder := range builders.Builders {
	//	serviceName := builder.Labels.Get("service_name")
	//	c.metrics.pprofsTotal.WithLabelValues(serviceName).Inc()
	//	c.metrics.pprofSamplesTotal.WithLabelValues(serviceName).Add(float64(len(builder.Profile.Sample)))
	//
	//	buf := bytes.NewBuffer(nil)
	//	_, err := builder.Write(buf)
	//	if err != nil {
	//		return fmt.Errorf("ebpf profile encode %w", err)
	//	}
	//	rawProfile := buf.Bytes()
	//
	//	appender := c.appendable.Appender()
	//	bytesSent += len(rawProfile)
	//	c.metrics.pprofBytesTotal.WithLabelValues(serviceName).Add(float64(len(rawProfile)))
	//
	//	samples := []*pyroscope.RawSample{{RawProfile: rawProfile}}
	//	err = appender.Append(context.Background(), builder.Labels, samples)
	//	if err != nil {
	//		level.Error(c.options.Logger).Log("msg", "ebpf pprof write", "err", err)
	//		continue
	//	}
	//}
	//level.Debug(c.options.Logger).Log("msg", "ebpf append done", "bytes_sent", bytesSent)
	return nil
}

type DebugInfo struct {
	Targets interface{} `alloy:"targets,attr,optional"`
}

func (c *Component) updateDebugInfo() {
	c.debugInfoLock.Lock()
	defer c.debugInfoLock.Unlock()

	c.debugInfo = DebugInfo{
		Targets: c.targetFinder.DebugInfo(),
	}
}

func targetsOptionFromArgs(args Arguments) sd.TargetsOptions {
	targets := make([]sd.DiscoveryTarget, 0, len(args.Targets))
	for _, t := range args.Targets {
		targets = append(targets, sd.DiscoveryTarget(t))
	}
	return sd.TargetsOptions{
		Targets:            targets,
		TargetsOnly:        true,
		ContainerCacheSize: args.ContainerIDCacheSize,
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

func createOTLPReporter(cfg *controller.Config) (*reporter.OTLPReporter, error) {
	intervals := times.New(cfg.MonitorInterval,
		cfg.ReporterInterval, cfg.ProbabilisticInterval)

	kernelVersion, err := helpers.GetKernelVersion()
	if err != nil {
		return nil, err
	}
	if cfg.CollAgentAddr == "" {
		return nil, fmt.Errorf("missing otlp collector address")
	}

	// hostname and sourceIP will be populated from the root namespace.
	hostname, sourceIP, err := helpers.GetHostnameAndSourceIP(cfg.CollAgentAddr)
	if err != nil {
		return nil, err
	}
	cfg.HostName, cfg.IPAddress = hostname, sourceIP

	return reporter.NewOTLP(&reporter.Config{
		CollAgentAddr:            cfg.CollAgentAddr,
		DisableTLS:               cfg.DisableTLS,
		MaxRPCMsgSize:            32 << 20, // 32 MiB
		MaxGRPCRetries:           5,
		GRPCOperationTimeout:     intervals.GRPCOperationTimeout(),
		GRPCStartupBackoffTime:   intervals.GRPCStartupBackoffTime(),
		GRPCConnectionTimeout:    intervals.GRPCConnectionTimeout(),
		ReportInterval:           intervals.ReportInterval(),
		ExecutablesCacheElements: 16384,
		// Next step: Calculate FramesCacheElements from numCores and samplingRate.
		FramesCacheElements: 65536,
		CGroupCacheElements: 1024,
		SamplesPerSecond:    cfg.SamplesPerSecond,
		KernelVersion:       kernelVersion,
		HostName:            hostname,
		IPAddress:           sourceIP,

		PyroscopeUsername:              cfg.PyroscopeUsername,
		PyroscopePasswordFile:          cfg.PyroscopePasswordFile,
		PyroscopeSymbCachePath:         cfg.PyroscopeSymbCachePath,
		PyroscopeSymbCacheSizeBytes:    cfg.PyroscopeSymbCacheSizeBytes,
		PyroscopeSymbolizeNativeFrames: cfg.PyroscopeSymbolizeNativeFrames,
	})
}
