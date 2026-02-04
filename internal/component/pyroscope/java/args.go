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
	Interval    time.Duration `alloy:"interval,attr,optional"`
	CPU         bool          `alloy:"cpu,attr,optional"`
	Event       string        `alloy:"event,attr,optional"`
	SampleRate  int           `alloy:"sample_rate,attr,optional"`
	Wall        string        `alloy:"wall,attr,optional"`
	AllUser     bool          `alloy:"all_user,attr,optional"`
	PerThread   bool          `alloy:"per_thread,attr,optional"`
	Filter      string        `alloy:"filter,attr,optional"`
	Sched       bool          `alloy:"sched,attr,optional"`
	TTSP        bool          `alloy:"ttsp,attr,optional"`
	Begin       string        `alloy:"begin,attr,optional"`
	End         string        `alloy:"end,attr,optional"`
	NoStop      bool          `alloy:"nostop,attr,optional"`
	Proc        string        `alloy:"proc,attr,optional"`
	TargetCPU   int           `alloy:"target_cpu,attr,optional"`
	RecordCPU   bool          `alloy:"record_cpu,attr,optional"`
	Alloc       string        `alloy:"alloc,attr,optional"`
	Live        bool          `alloy:"live,attr,optional"`
	NativeMem   string        `alloy:"native_mem,attr,optional"`
	NoFree      bool          `alloy:"no_free,attr,optional"`
	Lock        string        `alloy:"lock,attr,optional"`
	NativeLock  string        `alloy:"native_lock,attr,optional"`
	All         bool          `alloy:"all,attr,optional"`
	LogLevel    string        `alloy:"log_level,attr,optional"`
	Quiet       bool          `alloy:"quiet,attr,optional"`
	Include     []string      `alloy:"include,attr,optional"`
	Exclude     []string      `alloy:"exclude,attr,optional"`
	JStackDepth int           `alloy:"jstackdepth,attr,optional"`
	CStack      string        `alloy:"cstack,attr,optional"`
	Features    []string      `alloy:"features,attr,optional"`
	Trace       []string      `alloy:"trace,attr,optional"`
	JFRSync     string        `alloy:"jfrsync,attr,optional"`
	Signal      string        `alloy:"signal,attr,optional"`
	Clock       string        `alloy:"clock,attr,optional"`
}

func (rc *Arguments) UnmarshalAlloy(f func(any) error) error {
	*rc = DefaultArguments()
	type config Arguments
	return f((*config)(rc))
}

func DefaultArguments() Arguments {
	return Arguments{
		TmpDir: "/tmp",
		ProfilingConfig: ProfilingConfig{
			Interval:    60 * time.Second,
			CPU:         true,
			Event:       "itimer",
			SampleRate:  100,
			Wall:        "",
			AllUser:     false,
			PerThread:   false,
			Filter:      "",
			Sched:       false,
			TTSP:        false,
			Begin:       "",
			End:         "",
			NoStop:      false,
			Proc:        "",
			TargetCPU:   -1,
			RecordCPU:   false,
			Alloc:       "512k",
			Live:        false,
			NativeMem:   "",
			NoFree:      false,
			Lock:        "10ms",
			NativeLock:  "",
			All:         false,
			LogLevel:    "INFO",
			Quiet:       false,
			Include:     []string{},
			Exclude:     []string{},
			JStackDepth: 2048,
			CStack:      "",
			Features:    []string{},
			Trace:       []string{},
			JFRSync:     "",
			Signal:      "",
			Clock:       "",
		},
	}
}
