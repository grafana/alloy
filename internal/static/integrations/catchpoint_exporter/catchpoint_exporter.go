package catchpoint_exporter

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"

	collector "github.com/grafana/catchpoint-prometheus-exporter/collector"
)

// DefaultConfig is the default config for the catchpoint integration
var DefaultConfig = Config{
	VerboseLogging: false,
	WebhookPath:    "/catchpoint-webhook",
	Port:           "9090",
}

// Config is the configuration for the catchpoint integration
type Config struct {
	VerboseLogging bool   `yaml:"verbose_logging,omitempty"`
	WebhookPath    string `yaml:"webhook_path,omitempty"`
	Port           string `yaml:"port,omitempty"`
}

func (c *Config) exporterConfig() *collector.Config {
	return &collector.Config{
		VerboseLogging: c.VerboseLogging,
		WebhookPath:    c.WebhookPath,
		Port:           c.Port,
	}
}

// Identifier returns a string that identifies the integration.
func (c *Config) InstanceKey(agentKey string) (string, error) {
	return c.Port, nil
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration this config is for.
func (c *Config) Name() string {
	return "catchpoint"
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("catchpoint"))
}

// NewIntegration creates a new integration from the config.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	exporterConfig := c.exporterConfig()

	col := collector.NewCollector(l, exporterConfig)
	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(col),
	), nil
}
