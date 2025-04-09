package process

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
)

type Arguments struct {
	Join            []discovery.Target `alloy:"join,attr,optional"`
	RefreshInterval time.Duration      `alloy:"refresh_interval,attr,optional"`
	DiscoverConfig  DiscoverConfig     `alloy:"discover_config,block,optional"`
}

type DiscoverConfig struct {
	Cwd         bool `alloy:"cwd,attr,optional"`
	Exe         bool `alloy:"exe,attr,optional"`
	Commandline bool `alloy:"commandline,attr,optional"`
	Username    bool `alloy:"username,attr,optional"`
	UID         bool `alloy:"uid,attr,optional"`
	ContainerID bool `alloy:"container_id,attr,optional"`
	CgroupPath  bool `alloy:"cgroup_path,attr,optional"`
}

var DefaultConfig = Arguments{
	Join:            nil,
	RefreshInterval: 60 * time.Second,
	DiscoverConfig: DiscoverConfig{
		Cwd:         true,
		Exe:         true,
		Commandline: true,
		ContainerID: true,
		CgroupPath:  false,
	},
}

func (args *Arguments) SetToDefault() {
	*args = DefaultConfig
}
