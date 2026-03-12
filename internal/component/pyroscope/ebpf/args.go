package ebpf

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter"
)

// CommMode controls how the process comm (command name) is included in generated profiles.
type CommMode string

const (
	// CommModeNone disables including comm in profiles.
	CommModeNone CommMode = "none"
	// CommModeLabel adds comm as a pprof sample label.
	CommModeLabel CommMode = "label"
	// CommModeStackframe adds comm as the topmost stack frame.
	CommModeStackframe CommMode = "stackframe"
	// CommModeBoth adds comm as both a label and the topmost stack frame.
	CommModeBoth CommMode = "both"
)

func (m CommMode) toReporter() reporter.CommMode {
	return reporter.CommMode(m)
}

type Arguments struct {
	ForwardTo           []pyroscope.Appendable `alloy:"forward_to,attr"`
	Targets             []discovery.Target     `alloy:"targets,attr,optional"`
	CollectInterval     time.Duration          `alloy:"collect_interval,attr,optional"`
	SampleRate          int                    `alloy:"sample_rate,attr,optional"`
	PythonEnabled       bool                   `alloy:"python_enabled,attr,optional"`
	PerlEnabled         bool                   `alloy:"perl_enabled,attr,optional"`
	PHPEnabled          bool                   `alloy:"php_enabled,attr,optional"`
	HotspotEnabled      bool                   `alloy:"hotspot_enabled,attr,optional"`
	RubyEnabled         bool                   `alloy:"ruby_enabled,attr,optional"`
	V8Enabled           bool                   `alloy:"v8_enabled,attr,optional"`
	DotNetEnabled       bool                   `alloy:"dotnet_enabled,attr,optional"`
	GoEnabled           bool                   `alloy:"go_enabled,attr,optional"`
	Demangle            string                 `alloy:"demangle,attr,optional"`
	OffCPUThreshold     float64                `alloy:"off_cpu_threshold,attr,optional"`
	LoadProbe           bool                   `alloy:"load_probe,attr,optional"`
	UProbeLinks         []string               `alloy:"u_probe_links,attr,optional"`
	VerboseMode         bool                   `alloy:"verbose_mode,attr,optional"`
	LazyMode            bool                   `alloy:"lazy_mode,attr,optional"`
	DeprecatedArguments DeprecatedArguments    `alloy:",squash"`

	// undocumented
	PyroscopeDynamicProfilingPolicy bool   `alloy:"targets_only,attr,optional"`
	SymbCachePath                   string `alloy:"symb_cache_path,attr,optional"`
	SymbCacheSizeEntries            int    `alloy:"symb_cache_size,attr,optional"`
	ReporterUnsymbolizedStubs       bool   `alloy:"reporter_unsymbolized_stubs,attr,optional"`
	PIDLabel                        bool     `alloy:"pid_label,attr,optional"`
	Comm                            CommMode `alloy:"comm,attr,optional"`             // to address a Grafana Labs customer's escalation
	DropKernelFrames                bool     `alloy:"drop_kernel_frames,attr,optional"`
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
	// deprecated
	PIDMapSize int `alloy:"pid_map_size,attr,optional"`
	// deprecated
	CollectUserProfile bool `alloy:"collect_user_profile,attr,optional"`
	// deprecated
	CollectKernelProfile bool `alloy:"collect_kernel_profile,attr,optional"`
	// deprecated
	ContainerIDCacheSize uint32 `alloy:"container_id_cache_size,attr,optional"`
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	return nil
}
