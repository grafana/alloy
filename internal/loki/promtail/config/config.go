package config

import (
	"flag"
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/loki/promtail/client"
	"github.com/grafana/alloy/internal/loki/promtail/file"
	"github.com/grafana/alloy/internal/loki/promtail/limit"
	"github.com/grafana/alloy/internal/loki/promtail/positions"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/grafana/alloy/internal/loki/promtail/server"
	"github.com/grafana/alloy/internal/loki/promtail/tracing"
	"github.com/grafana/alloy/internal/loki/promtail/wal"
)

// Options contains cross-cutting promtail configurations
type Options struct {
}

// Config for promtail, describing what files to watch.
type Config struct {
	Global       GlobalConfig  `yaml:"global,omitempty"`
	ServerConfig server.Config `yaml:"server,omitempty"`
	// deprecated use ClientConfigs instead
	ClientConfig    client.Config         `yaml:"client,omitempty"`
	ClientConfigs   []client.Config       `yaml:"clients,omitempty"`
	PositionsConfig positions.Config      `yaml:"positions,omitempty"`
	ScrapeConfig    []scrapeconfig.Config `yaml:"scrape_configs,omitempty"`
	TargetConfig    file.Config           `yaml:"target_config,omitempty"`
	LimitsConfig    limit.Config          `yaml:"limits_config,omitempty"`
	Options         Options               `yaml:"options,omitempty"`
	Tracing         tracing.Config        `yaml:"tracing"`
	WAL             wal.Config            `yaml:"wal"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	// We want to set c to the defaults and then overwrite it with the input.
	// To make unmarshal fill the plain data struct rather than calling UnmarshalYAML
	// again, we have to hide it using a type indirection.
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	// Validate unique names.
	jobNames := map[string]struct{}{}
	for _, j := range c.ScrapeConfig {
		if _, ok := jobNames[j.JobName]; ok {
			return fmt.Errorf("found multiple scrape configs with job name %q", j.JobName)
		}
		jobNames[j.JobName] = struct{}{}
	}
	return nil
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (c *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	c.Global.RegisterFlagsWithPrefix(prefix, f)
	c.ServerConfig.RegisterFlagsWithPrefix(prefix, f)
	c.ClientConfig.RegisterFlagsWithPrefix(prefix, f)
	c.PositionsConfig.RegisterFlagsWithPrefix(prefix, f)
	c.TargetConfig.RegisterFlagsWithPrefix(prefix, f)
	c.LimitsConfig.RegisterFlagsWithPrefix(prefix, f)
	c.Tracing.RegisterFlagsWithPrefix(prefix, f)
}

// RegisterFlags registers flags.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("", f)
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}

// GlobalConfig holds configuration settings which apply to all targets.
// Individual scrape jobs can override the defaults.
type GlobalConfig struct {
	FileWatch file.WatchConfig `mapstructure:"file_watch_config" yaml:"file_watch_config"`
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (cfg *GlobalConfig) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	cfg.FileWatch.RegisterFlagsWithPrefix(prefix+"file-watch.", f)
}

// RegisterFlags register flags.
func (cfg *GlobalConfig) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}
