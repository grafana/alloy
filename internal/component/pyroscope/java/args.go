package java

import (
	"fmt"
	"regexp"
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
	Interval        time.Duration `alloy:"interval,attr,optional"`
	SampleRate      int           `alloy:"sample_rate,attr,optional"`
	Alloc           string        `alloy:"alloc,attr,optional"`
	Lock            string        `alloy:"lock,attr,optional"`
	CPU             bool          `alloy:"cpu,attr,optional"`
	Event           string        `alloy:"event,attr,optional"`
	PerThread       bool          `alloy:"per_thread,attr,optional"`
	LogLevel        string        `alloy:"log_level,attr,optional"`
	Quiet           bool          `alloy:"quiet,attr,optional"`
	CustomArguments []string      `alloy:"custom_arguments,attr,optional"`

	Thread ThreadConfig `alloy:"thread,block,optional"`
}

// ThreadConfig surfaces the sampled thread in the profile. Requires per_thread.
// frame renders the thread name as a root frame so flame graphs split by thread;
// label_name adds a sample label under that name for filtering/grouping; regex,
// when set, collapses the thread name to its first capture group (e.g. a pool
// name) and applies to both frame and label.
type ThreadConfig struct {
	Frame     bool   `alloy:"frame,attr,optional"`
	LabelName string `alloy:"label_name,attr,optional"`
	Regex     string `alloy:"regex,attr,optional"`
}

func (rc *Arguments) UnmarshalAlloy(f func(any) error) error {
	*rc = DefaultArguments()
	type config Arguments
	return f((*config)(rc))
}

func (arg *Arguments) Validate() error {
	t := arg.ProfilingConfig.Thread
	if t.Regex != "" {
		re, err := regexp.Compile(t.Regex)
		if err != nil {
			return fmt.Errorf("invalid thread.regex: %w", err)
		}
		if re.NumSubexp() < 1 {
			return fmt.Errorf("thread.regex must contain at least one capture group")
		}
	}
	return nil
}

func DefaultArguments() Arguments {
	return Arguments{
		TmpDir: "/tmp",
		ProfilingConfig: ProfilingConfig{
			Interval:        60 * time.Second,
			SampleRate:      100,
			Alloc:           "512k",
			Lock:            "10ms",
			CPU:             true,
			Event:           "itimer",
			PerThread:       false,
			LogLevel:        "INFO",
			Quiet:           false,
			CustomArguments: []string{},
		},
	}
}
