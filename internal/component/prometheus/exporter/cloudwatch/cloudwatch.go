package cloudwatch

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/cloudwatch_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.cloudwatch",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "cloudwatch"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	exporterConfig, err := ConvertToYACE(a, opts.SLogger)
	if err != nil {
		return nil, "", fmt.Errorf("invalid cloudwatch exporter configuration: %w", err)
	}
	// yaceSess expects a default value of True
	fipsEnabled := !a.FIPSDisabled

	if !a.UseAWSSDKVersion2 {
		opts.SLogger.Warn(
			"the `aws_sdk_version_v2` argument is deprecated and has no effect, AWS SDK for Go v2 is always used - remove this argument from your configuration",
		)
	}

	if a.DecoupledScrape.Enabled {
		exp, err := cloudwatch_exporter.NewDecoupledCloudwatchExporter(opts.ID, opts.SLogger, exporterConfig, a.DecoupledScrape.ScrapeInterval, fipsEnabled, a.LabelsSnakeCase)
		return exp, getHash(a), err
	}

	exp, err := cloudwatch_exporter.NewCloudwatchExporter(opts.ID, opts.SLogger, exporterConfig, fipsEnabled, a.LabelsSnakeCase)
	return exp, getHash(a), err
}
