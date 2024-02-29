package statsd

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.statsd",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "statsd"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	cfg, err := a.Convert()
	if err != nil {
		return nil, "", err
	}
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, cfg, defaultInstanceKey)
}
