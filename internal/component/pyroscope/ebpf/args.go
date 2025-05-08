package ebpf

import (
	"errors"
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
)

type Arguments struct {
	ForwardTo            []pyroscope.Appendable `alloy:"forward_to,attr"`
	Targets              []discovery.Target     `alloy:"targets,attr,optional"`
	CollectInterval      time.Duration          `alloy:"collect_interval,attr,optional"`
	SampleRate           int                    `alloy:"sample_rate,attr,optional"`
	PidCacheSize         int                    `alloy:"pid_cache_size,attr,optional"`
	BuildIDCacheSize     int                    `alloy:"build_id_cache_size,attr,optional"`
	SameFileCacheSize    int                    `alloy:"same_file_cache_size,attr,optional"`
	ContainerIDCacheSize int                    `alloy:"container_id_cache_size,attr,optional"`
	CacheRounds          int                    `alloy:"cache_rounds,attr,optional"`
	CollectUserProfile   bool                   `alloy:"collect_user_profile,attr,optional"`
	CollectKernelProfile bool                   `alloy:"collect_kernel_profile,attr,optional"`
	Demangle             string                 `alloy:"demangle,attr,optional"`
	GoTableFallback      bool                   `alloy:"go_table_fallback,attr,optional"`
	PythonEnabled        bool                   `alloy:"python_enabled,attr,optional"`
	SymbolsMapSize       int                    `alloy:"symbols_map_size,attr,optional"`
	PIDMapSize           int                    `alloy:"pid_map_size,attr,optional"`
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	var errs []error
	if arg.SymbolsMapSize <= 0 {
		errs = append(errs, errors.New("symbols_map_size must be greater than 0"))
	}
	if arg.PIDMapSize <= 0 {
		errs = append(errs, errors.New("pid_map_size must be greater than 0"))
	}
	return errors.Join(errs...)
}
