package ssh_exporter

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/prometheus/client_golang/prometheus"
)

func (c *Config) Name() string {
	return "ssh_exporter"
}

func (c *Config) InstanceKey(agentKey string) (string, error) {
	return "ssh_exporter", nil
}

func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	// Use the provided logger (global filters control verbosity)

	var collectors []prometheus.Collector

	// Create collectors for each target.
	for _, target := range c.Targets {
		collector, err := NewSSHCollector(logger, target)
		if err != nil {
			return nil, err
		}
		collectors = append(collectors, collector)
	}

	return integrations.NewCollectorIntegration(
		c.Name(), integrations.WithCollectors(collectors...),
	), nil
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("ssh_exporter"))
}
