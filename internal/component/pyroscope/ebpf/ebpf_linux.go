//go:build linux && (arm64 || amd64) && pyroscope_ebpf

package ebpf

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/oklog/run"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/interpreter/python"
	discovery2 "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"
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
	python.NoContinueWithNextUnwinder.Store(true)
}

func New(opts component.Options, args Arguments) (component.Component, error) {
	cfg, err := args.Convert()
	if err != nil {
		return nil, err
	}
	cgroups, err := reporter.NewContainerIDCache(args.ContainerIDCacheSize)
	if err != nil {
		return nil, err
	}
	dynamicProfilingPolicy := cfg.PyroscopeDynamicProfilingPolicy
	discovery := discovery2.NewTargetProducer(cgroups, args.targetsOptions(dynamicProfilingPolicy))
	ms := newMetrics(opts.Registerer)

	appendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)

	var nfs samples.NativeSymbolResolver
	if cfg.SymbolizeNativeFrames {
		tf := irsymcache.NewTableFactory()
		nfs, err = irsymcache.NewFSCache(tf, irsymcache.Options{
			SizeEntries: uint32(cfg.SymbCacheSizeEntries),
			Path:        cfg.SymbCachePath,
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
		argsUpdate:             make(chan Arguments, 4),
	}

	cfg.Reporter, err = reporter.New(opts.Logger, cgroups, cfg, discovery, nfs, reporter.PPROFConsumerFunc(func(ctx context.Context, ps []reporter.PPROF) {
		res.sendProfiles(ctx, ps)
	}))
	if err != nil {
		return nil, err
	}
	if cfg.VerboseMode {
		logrus.SetLevel(logrus.DebugLevel)
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
		PerlEnabled:          true,
		PHPEnabled:           true,
		HotspotEnabled:       true,
		RubyEnabled:          true,
		V8Enabled:            true,
		DotNetEnabled:        true,
		GoEnabled:            false,
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
	targetFinder           discovery2.TargetProducer

	metrics *metrics
	cfg     *controller.Config

	healthMut sync.RWMutex
	health    component.Health
}

func (c *Component) Run(ctx context.Context) error {

	c.checkTraceFS()
	ctlr := controller.New(c.cfg)
	const sessionMaxErrors = 3
	var err error
	for i := 0; i < sessionMaxErrors; i++ {
		err = ctlr.Start(ctx)
		if err != nil {
			c.reportUnhealthy(err)
			time.Sleep(c.cfg.ReporterInterval)
			continue
		}
		defer ctlr.Shutdown()
		c.reportHealthy()
		break
	}
	if err != nil {
		return err
	}

	var g run.Group
	g.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case newArgs := <-c.argsUpdate:
				c.args = newArgs
				c.targetFinder.Update(c.args.targetsOptions(c.dynamicProfilingPolicy))
				c.appendable.UpdateChildren(newArgs.ForwardTo)
			}
		}
	}, func(error) {})
	return g.Run()
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	select {
	case c.argsUpdate <- newArgs:
	default:
		_ = level.Debug(c.options.Logger).Log("msg", "dropped args update")
	}
	return nil
}

func (c *Component) reportUnhealthy(err error) {
	_ = level.Error(c.options.Logger).
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
		level.Debug(c.options.Logger).Log("msg", "found tracefs at "+p)
		return
	}
	mountPath := candidates[0]
	err := syscall.Mount("tracefs", mountPath, "tracefs", 0, "")
	if err != nil {
		level.Error(c.options.Logger).Log("msg", "failed to mount tracefs at "+mountPath, "err", err)
	} else {
		level.Debug(c.options.Logger).Log("msg", "mounted tracefs at "+mountPath)
	}
}
