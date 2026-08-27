package beyla

import (
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/config"
	"github.com/grafana/alloy/internal/component/discovery"
)

type Arguments = config.Arguments

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}
