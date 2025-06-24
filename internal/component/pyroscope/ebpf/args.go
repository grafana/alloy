package ebpf

import (
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	discovery2 "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
)

type Arguments struct {
	ForwardTo            []pyroscope.Appendable `alloy:"forward_to,attr"`
	Targets              []discovery.Target     `alloy:"targets,attr,optional"`
	CollectInterval      time.Duration          `alloy:"collect_interval,attr,optional"`
	SampleRate           int                    `alloy:"sample_rate,attr,optional"`
	CollectUserProfile   bool                   `alloy:"collect_user_profile,attr,optional"`
	CollectKernelProfile bool                   `alloy:"collect_kernel_profile,attr,optional"`
	PythonEnabled        bool                   `alloy:"python_enabled,attr,optional"`
	PerlEnabled          bool                   `alloy:"perl_enabled,attr,optional"`
	PHPEnabled           bool                   `alloy:"php_enabled,attr,optional"`
	HotspotEnabled       bool                   `alloy:"hotspot_enabled,attr,optional"`
	RubyEnabled          bool                   `alloy:"ruby_enabled,attr,optional"`
	V8Enabled            bool                   `alloy:"v8_enabled,attr,optional"`
	DotNetEnabled        bool                   `alloy:"dotnet_enabled,attr,optional"`
	GoEnabled            bool                   `alloy:"go_enabled,attr,optional"`
	PIDMapSize           int                    `alloy:"pid_map_size,attr,optional"`
	Demangle             string                 `alloy:"demangle,attr,optional"`
	ContainerIDCacheSize uint32                 `alloy:"container_id_cache_size,attr,optional"`
	DeprecatedArguments  DeprecatedArguments    `alloy:",squash"`
}

type DeprecatedArguments struct {
	// deprecated
	PidCacheSize int `alloy:"pid_cache_size,attr,optional"`
	// deprecated
	BuildIDCacheSize int `alloy:"build_id_cache_size,attr,optional"`
	// deprecated
	SameFileCacheSize int `alloy:"same_file_cache_size,attr,optional"`
	// deprecated
	CacheRounds int `alloy:"cache_rounds,attr,optional"`
	// deprecated
	GoTableFallback bool `alloy:"go_table_fallback,attr,optional"`
	// deprecated
	SymbolsMapSize int `alloy:"symbols_map_size,attr,optional"`
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	return nil
}

func (args *Arguments) Convert() (*controller.Config, error) {
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
	cfg.Tracers = args.tracers()
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

func (args Arguments) targetsOptions(dynamicProfilingPolicy bool) discovery2.TargetsOptions {
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
