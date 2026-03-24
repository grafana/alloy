package statsd

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.statsd",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "statsd"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	cfg, err := a.Convert()
	if err != nil {
		return nil, "", err
	}
	defaultInstanceKey := opts.ID // use component ID if there is no better option
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, cfg, defaultInstanceKey)
}
