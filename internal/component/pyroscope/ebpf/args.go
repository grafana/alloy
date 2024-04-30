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
	VerifierLogSize      int                    `alloy:"verifier_log_size,attr,optional"`
	PidCacheSize         int                    `alloy:"pid_cache_size,attr,optional"`
	BuildIDCacheSize     int                    `alloy:"build_id_cache_size,attr,optional"`
	SameFileCacheSize    int                    `alloy:"same_file_cache_size,attr,optional"`
	ContainerIDCacheSize int                    `alloy:"container_id_cache_size,attr,optional"`
	CacheRounds          int                    `alloy:"cache_rounds,attr,optional"`
	CollectUserProfile   bool                   `alloy:"collect_user_profile,attr,optional"`
	CollectKernelProfile bool                   `alloy:"collect_kernel_profile,attr,optional"`
	Demangle             string                 `alloy:"demangle,attr,optional"`
	PythonEnabled        bool                   `alloy:"python_enabled,attr,optional"`
}
