package cloudwatch

import (
	"fmt"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/cloudwatch_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.cloudwatch",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "cloudwatch"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	exporterConfig, err := ConvertToYACE(a)
	if err != nil {
		return nil, "", fmt.Errorf("invalid cloudwatch exporter configuration: %w", err)
	}
	// yaceSess expects a default value of True
	fipsEnabled := !a.FIPSDisabled

	if a.DecoupledScrape.Enabled {
		return cloudwatch_exporter.NewDecoupledCloudwatchExporter(opts.ID, opts.Logger, exporterConfig, a.DecoupledScrape.ScrapeInterval, fipsEnabled, a.Debug), getHash(a), nil
	}

	return cloudwatch_exporter.NewCloudwatchExporter(opts.ID, opts.Logger, exporterConfig, fipsEnabled, a.Debug), getHash(a), nil
}
