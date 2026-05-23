package nvidiagpu_exporter

import (
	"context"
	"log/slog"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/nvidiagpu_exporter/internal/exporter"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig holds the default configuration for the nvidiagpu_exporter integration.
var DefaultConfig = Config{
	NvidiaSmiCommand: "nvidia-smi",
}

// Config controls the nvidiagpu_exporter integration.
type Config struct {
	NvidiaSmiCommand string `yaml:"nvidia_smi_command,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration this config is for.
func (c *Config) Name() string {
	return "nvidiagpu_exporter"
}

// InstanceKey returns the instance key for the integration.
func (c *Config) InstanceKey(_ string) (string, error) {
	return "nvidiagpu", nil
}

// NewIntegration converts the config into an integration instance.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("nvidiagpu"))
}

// New creates a new nvidiagpu_exporter integration.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	level.Debug(logger).Log("msg", "initializing nvidiagpu_exporter", "config", c)

	e, err := exporter.New(
		context.Background(),
		nil,
		"nvidia_smi",
		c.NvidiaSmiCommand,
		"AUTO",
		slog.Default(),
	)
	if err != nil {
		return nil, err
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(e),
	), nil
}
