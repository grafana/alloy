package process

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
)

type Arguments struct {
	Join            []discovery.Target `river:"join,attr,optional"`
	RefreshInterval time.Duration      `river:"refresh_interval,attr,optional"`
	DiscoverConfig  DiscoverConfig     `river:"discover_config,block,optional"`
}

type DiscoverConfig struct {
	Cwd         bool `river:"cwd,attr,optional"`
	Exe         bool `river:"exe,attr,optional"`
	Commandline bool `river:"commandline,attr,optional"`
	Username    bool `river:"username,attr,optional"`
	UID         bool `river:"uid,attr,optional"`
	ContainerID bool `river:"container_id,attr,optional"`
}

var DefaultConfig = Arguments{
	Join:            nil,
	RefreshInterval: 60 * time.Second,
	DiscoverConfig: DiscoverConfig{
		Cwd:         true,
		Exe:         true,
		Commandline: true,
		ContainerID: true,
	},
}

func (args *Arguments) SetToDefault() {
	*args = DefaultConfig
}
