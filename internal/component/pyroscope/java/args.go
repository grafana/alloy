package java

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
)

type Arguments struct {
	Targets   []discovery.Target     `alloy:"targets,attr"`
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`

	TmpDir          string          `alloy:"tmp_dir,attr,optional"`
	ProfilingConfig ProfilingConfig `alloy:"profiling_config,block,optional"`
}

type ProfilingConfig struct {
	Interval   time.Duration `alloy:"interval,attr,optional"`
	SampleRate int           `alloy:"sample_rate,attr,optional"`
	Alloc      string        `alloy:"alloc,attr,optional"`
	Lock       string        `alloy:"lock,attr,optional"`
	CPU        bool          `alloy:"cpu,attr,optional"`
	Wall       bool          `alloy:"wall,attr,optional"`
}

func (rc *Arguments) UnmarshalAlloy(f func(interface{}) error) error {
	*rc = defaultArguments()
	type config Arguments
	return f((*config)(rc))
}

func defaultArguments() Arguments {
	return Arguments{
		TmpDir: "/tmp",
		ProfilingConfig: ProfilingConfig{
			Interval:   60 * time.Second,
			SampleRate: 100,
			Alloc:      "10ms",
			Lock:       "512k",
			CPU:        true,
			Wall:       false,
		},
	}
}
