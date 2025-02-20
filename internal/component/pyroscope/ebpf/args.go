package ebpf

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
)

type Arguments struct {
	ForwardTo            []pyroscope.Appendable `alloy:"forward_to,attr"`
	Targets              []discovery.Target     `alloy:"targets,attr,optional"`
	CollectInterval      time.Duration          `alloy:"collect_interval,attr,optional"`
	SampleRate           int                    `alloy:"sample_rate,attr,optional"`
	CollectUserProfile   bool                   `alloy:"collect_user_profile,attr,optional"`
	CollectKernelProfile bool                   `alloy:"collect_kernel_profile,attr,optional"`
	PythonEnabled        bool                   `alloy:"python_enabled,attr,optional"`
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
