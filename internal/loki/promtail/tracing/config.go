package tracing

import "flag"

type Config struct {
	Enabled bool `yaml:"enabled"`
}

func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.BoolVar(&cfg.Enabled, prefix+"tracing.enabled", true, "Set to false to disable tracing.")
}
