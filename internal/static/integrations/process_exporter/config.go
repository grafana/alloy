// Package process_exporter embeds https://github.com/ncabatoff/process-exporter
package process_exporter

import (
	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"

	exporter_config "github.com/ncabatoff/process-exporter/config"
)

// DefaultConfig holds the default settings for the process_exporter integration.
var DefaultConfig = Config{
	ProcFSPath: "/proc",
	Children:   true,
	Threads:    true,
	SMaps:      true,
	Recheck:    false,
}

// Config controls the process_exporter integration.
type Config struct {
	ProcessExporter exporter_config.MatcherRules `yaml:"process_names,omitempty"`

	ProcFSPath string `yaml:"procfs_path,omitempty"`
	Children   bool   `yaml:"track_children,omitempty"`
	Threads    bool   `yaml:"track_threads,omitempty"`
	SMaps      bool   `yaml:"gather_smaps,omitempty"`
	Recheck    bool   `yaml:"recheck_on_scrape,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "process_exporter"
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeSingleton, metricsutils.NewNamedShim("process"))
}
