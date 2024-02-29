package java

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/pyroscope"
)

type Arguments struct {
	Targets   []discovery.Target     `river:"targets,attr"`
	ForwardTo []pyroscope.Appendable `river:"forward_to,attr"`

	TmpDir          string          `river:"tmp_dir,attr,optional"`
	ProfilingConfig ProfilingConfig `river:"profiling_config,block,optional"`
}

type ProfilingConfig struct {
	Interval   time.Duration `river:"interval,attr,optional"`
	SampleRate int           `river:"sample_rate,attr,optional"`
	Alloc      string        `river:"alloc,attr,optional"`
	Lock       string        `river:"lock,attr,optional"`
	CPU        bool          `river:"cpu,attr,optional"`
}

func (rc *Arguments) UnmarshalRiver(f func(interface{}) error) error {
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
		},
	}
}
