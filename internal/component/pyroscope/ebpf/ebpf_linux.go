//go:build linux && (arm64 || amd64)

package ebpf

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/pyroscope/lidia"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/interpreter/python"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	ebpfmetrics "go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/process"
	discovery2 "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"
	reporter2 "go.opentelemetry.io/ebpf-profiler/reporter"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.ebpf",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			arguments := args.(Arguments)

			return New(opts.Logger, opts.Registerer, opts.ID, arguments)
		},
	})
	python.NoContinueWithNextUnwinder.Store(true)
	// Disable ebpf profiler metrics
	ebpfmetrics.Start(metricnoop.Meter{})
}

func New(logger log.Logger, reg prometheus.Registerer, id string, args Arguments) (*Component, error) {
	cfg, err := args.Convert()
	if err != nil {
		return nil, err
	}
	dynamicProfilingPolicy := args.PyroscopeDynamicProfilingPolicy
	discovery := discovery2.NewTargetProducer(args.targetsOptions(dynamicProfilingPolicy))
	ms := newMetrics(reg)

	appendable := pyroscope.NewFanout(args.ForwardTo, id, reg)

	var symbols *irsymcache.Resolver
	if args.DebugInfoOptions.OnTargetSymbolizationEnabled {
		symbols, err = irsymcache.NewFSCache(irsymcache.TableTableFactory{
			Options: []lidia.Option{
				lidia.WithFiles(),
				lidia.WithLines(),
			},
		}, irsymcache.Options{
			SizeEntries: uint32(args.SymbCacheSizeEntries),
			Path:        args.SymbCachePath,
		})
		if err != nil {
			return nil, err
		}
	}

	if dynamicProfilingPolicy {
		cfg.Policy = &dynamicprofiling.ServiceDiscoveryTargetsOnlyPolicy{Discovery: discovery}
	} else {
		cfg.Policy = dynamicprofiling.AlwaysOnPolicy{}
	}

	res := &Component{
		cfg:                    cfg,
		logger:                 logger,
		metrics:                ms,
		appendable:             appendable,
		args:                   args,
		targetFinder:           discovery,
		dynamicProfilingPolicy: dynamicProfilingPolicy,
		argsUpdate:             make(chan Arguments, 4),
		symbols:                symbols,
	}

	r := reporter.NewPPROF(logger, &reporter.Config{
		ReportInterval:            cfg.ReporterInterval,
		SamplesPerSecond:          int64(cfg.SamplesPerSecond),
		Demangle:                  args.Demangle,
		ReporterUnsymbolizedStubs: args.ReporterUnsymbolizedStubs,
	}, discovery,
		symbols,
		func(ctx context.Context, ps []reporter.PPROF) {
			res.sendProfiles(ctx, ps)
		})

	cfg.Reporter = r
	cfg.ExecutableReporter = res

	if cfg.VerboseMode {
		logrus.SetLevel(logrus.DebugLevel)
	}

	return res, nil
}

type Component struct {
	logger                 log.Logger
	args                   Arguments
	dynamicProfilingPolicy bool
	argsUpdate             chan Arguments
	appendable             *pyroscope.Fanout
	targetFinder           discovery2.TargetProducer

	metrics *metrics
	cfg     *controller.Config

	healthMut sync.RWMutex
	health    component.Health
	symbols   *irsymcache.Resolver
}

func (c *Component) Run(ctx context.Context) error {
	c.checkTraceFS()

	if c.args.LazyMode && len(c.args.Targets) == 0 {
		_ = level.Info(c.logger).Log("msg", "lazy mode enabled, waiting for targets to profile")
		if err := c.waitForTargets(ctx); err != nil {
			return err
		}
	}

	ctlr := controller.New(c.cfg)
	const sessionMaxErrors = 3
	var err error
	for i := 0; i < sessionMaxErrors; i++ {
		err = ctlr.Start(ctx)
		if err != nil {
			c.reportUnhealthy(err)
			c.metrics.profilingSessionsFailingTotal.Inc()
			time.Sleep(c.cfg.ReporterInterval)
			continue
		}
		break
	}
	if err != nil {
		return err
	}
	c.reportHealthy()
	c.metrics.profilingSessionsTotal.Inc()
	defer func() {
		ctlr.Shutdown()
		if c.cfg.ExecutableReporter != nil {
			if nfs, ok := c.cfg.ExecutableReporter.(*irsymcache.Resolver); ok {
				nfs.Cleanup()
			}
		}
	}()

	var g run.Group

	g.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case newArgs := <-c.argsUpdate:
				c.updateArgs(newArgs)
			}
		}
	}, func(error) {})
	return g.Run()
}

func (c *Component) updateArgs(newArgs Arguments) {
	c.args = newArgs
	c.targetFinder.Update(c.args.targetsOptions(c.dynamicProfilingPolicy))
	c.appendable.UpdateChildren(newArgs.ForwardTo)
	c.metrics.targetsActive.Set(float64(len(c.args.Targets)))
}

func (c *Component) waitForTargets(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case newArgs := <-c.argsUpdate:
			c.updateArgs(newArgs)
			if len(c.args.Targets) > 0 {
				return nil
			}
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	select {
	case c.argsUpdate <- newArgs:
	default:
		_ = level.Debug(c.logger).Log("msg", "dropped args update")
	}
	return nil
}

func (c *Component) reportUnhealthy(err error) {
	_ = level.Error(c.logger).
		Log("msg", "unhealthy", "err", err)

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

func (c *Component) checkTraceFS() {
	candidates := []string{
		"/sys/kernel/tracing",
		"/sys/kernel/debug/tracing",
	}
	for _, p := range candidates {
		_, err := os.Stat(filepath.Join(p, "events"))
		if err != nil {
			continue
		}
		level.Debug(c.logger).Log("msg", "found tracefs at "+p)
		return
	}
	mountPath := candidates[0]
	err := syscall.Mount("tracefs", mountPath, "tracefs", 0, "")
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to mount tracefs at "+mountPath, "err", err)
	} else {
		level.Debug(c.logger).Log("msg", "mounted tracefs at "+mountPath)
	}
}

func (c *Component) ReportExecutable(md *reporter2.ExecutableMetadata) {
	if md.MappingFile == (libpf.FrameMappingFile{}) {
		return
	}
	if c.symbols != nil {
		c.symbols.ReportExecutable(md)
	}
	if c.args.DebugInfoOptions.UploadEnabled {
		c.reportExecutableForDebugInfoUpload(md)
	}
}

func (c *Component) reportExecutableForDebugInfoUpload(args *reporter2.ExecutableMetadata) {
	extractAsFile := func(pid libpf.PID, file string) string {
		return path.Join("/proc", strconv.Itoa(int(pid)), "root", file)
	}
	mf := args.MappingFile.Value()
	open := func() (process.ReadAtCloser, error) {
		fallback := func() (process.ReadAtCloser, error) {
			return args.Process.OpenMappingFile(args.Mapping)
		}
		if args.DebuglinkFileName == "" {
			return fallback()
		}
		file := extractAsFile(args.Process.PID(), args.DebuglinkFileName)
		if f, err := os.Open(file); err != nil {
			return fallback()
		} else {
			return f, nil
		}
	}
	c.appendable.Upload(debuginfo.UploadJob{
		FrameMappingFileData: mf,
		Open:                 open,
		InitArguments:        c.args.DebugInfoOptions,
	})
}

// NewDefaultArguments create the default settings for a scrape job.
func NewDefaultArguments() Arguments {
	return Arguments{
		CollectInterval: 15 * time.Second,
		SampleRate:      19,
		Demangle:        "none",
		PythonEnabled:   true,
		PerlEnabled:     true,
		PHPEnabled:      true,
		HotspotEnabled:  true,
		RubyEnabled:     true,
		V8Enabled:       true,
		DotNetEnabled:   true,
		OffCPUThreshold: 0,
		GoEnabled:       true,
		LoadProbe:       false,
		UProbeLinks:     []string{},
		VerboseMode:     false,
		LazyMode:        false,

		DebugInfoOptions: debuginfo.Arguments{
			OnTargetSymbolizationEnabled: true,
			UploadEnabled:                false,
			CacheSize:                    65536,
			StripTextSection:             false,
			QueueSize:                    1024,
			WorkerNum:                    8,
		},

		// undocumented
		PyroscopeDynamicProfilingPolicy: true,
		SymbCachePath:                   "/tmp/symb-cache",
		SymbCacheSizeEntries:            2048,
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = NewDefaultArguments()
}

func (args *Arguments) Convert() (*controller.Config, error) {
	cfgProtoType, err := controller.ParseArgs()
	if err != nil {
		return nil, err
	}

	if err = cfgProtoType.Validate(); err != nil {
		return nil, err
	}

	cfg := &controller.Config{Config: cfgProtoType}
	cfg.SendErrorFrames = true
	cfg.ReporterInterval = args.CollectInterval
	cfg.SamplesPerSecond = args.SampleRate
	cfg.Tracers = args.tracers()
	cfg.OffCPUThreshold = args.OffCPUThreshold
	cfg.LoadProbe = args.LoadProbe
	cfg.ProbeLinks = args.UProbeLinks
	cfg.VerboseMode = args.VerboseMode
	return cfg, nil
}

func (args *Arguments) tracers() string {
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
	if args.GoEnabled {
		tracers = append(tracers, "go")
	}
	return strings.Join(tracers, ",")
}

func (args *Arguments) targetsOptions(dynamicProfilingPolicy bool) discovery2.TargetsOptions {
	targets := make([]discovery2.DiscoveredTarget, 0, len(args.Targets))
	for _, t := range args.Targets {
		targets = append(targets, t.AsMap())
	}
	return discovery2.TargetsOptions{
		Targets:     targets,
		TargetsOnly: dynamicProfilingPolicy,
		DefaultTarget: discovery2.DiscoveredTarget{
			"service_name": "ebpf/unspecified",
		},
	}
}
