package server

import (
	"flag"

	serverww "github.com/grafana/dskit/server"
)

// Config extends weaveworks server config
type Config struct {
	serverww.Config   `yaml:",inline"`
	ExternalURL       string `yaml:"external_url"`
	HealthCheckTarget *bool  `yaml:"health_check_target"`
	Disable           bool   `yaml:"disable"`
	ProfilingEnabled  bool   `yaml:"profiling_enabled"`
	Reload            bool   `yaml:"enable_runtime_reload"`
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	// NOTE: weaveworks server's config can't be registered with a prefix.
	cfg.Config.RegisterFlags(f)

	f.BoolVar(&cfg.Disable, prefix+"server.disable", false, "Disable the http and grpc server.")
	f.BoolVar(&cfg.ProfilingEnabled, prefix+"server.profiling_enabled", false, "Enable the /debug/fgprof and /debug/pprof endpoints for profiling.")
	f.BoolVar(&cfg.Reload, prefix+"server.enable-runtime-reload", false, "Enable reload via HTTP request.")
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", f)
}
