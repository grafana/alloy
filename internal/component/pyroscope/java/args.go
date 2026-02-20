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

	// undocumented
	Dist string `alloy:"dist,attr,optional"`
}

type ProfilingConfig struct {
	Interval       time.Duration `alloy:"interval,attr,optional"`
	SampleRate     int           `alloy:"sample_rate,attr,optional"`
	Alloc          string        `alloy:"alloc,attr,optional"`
	Lock           string        `alloy:"lock,attr,optional"`
	CPU            bool          `alloy:"cpu,attr,optional"`
	Event          string        `alloy:"event,attr,optional"`
	PerThread      bool          `alloy:"per_thread,attr,optional"`
	LogLevel       string        `alloy:"log_level,attr,optional"`
	Quiet          bool          `alloy:"quiet,attr,optional"`
	ExtraArguments []string      `alloy:"extra_arguments,attr,optional"`
}

func (rc *Arguments) UnmarshalAlloy(f func(any) error) error {
	*rc = DefaultArguments()
	type config Arguments
	return f((*config)(rc))
}

func (arg *Arguments) Validate() error {
	return nil
}

func DefaultArguments() Arguments {
	return Arguments{
		TmpDir: "/tmp",
		ProfilingConfig: ProfilingConfig{
			Interval:       60 * time.Second,
			SampleRate:     100,
			Alloc:          "512k",
			Lock:           "10ms",
			CPU:            true,
			Event:          "itimer",
			PerThread:      false,
			LogLevel:       "INFO",
			Quiet:          false,
			ExtraArguments: []string{},
		},
	}
}
