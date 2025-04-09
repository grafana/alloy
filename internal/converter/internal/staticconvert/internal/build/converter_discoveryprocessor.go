package build

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcol_discovery "github.com/grafana/alloy/internal/component/otelcol/processor/discovery"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	prometheus_component "github.com/grafana/alloy/internal/converter/internal/prometheusconvert/component"
	"github.com/grafana/alloy/internal/static/traces/promsdprocessor"
	prom_config "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"
)

func init() {
	converters = append(converters, discoveryProcessorConverter{})
}

type discoveryProcessorConverter struct{}

func (discoveryProcessorConverter) Factory() component.Factory {
	return promsdprocessor.NewFactory()
}

func (discoveryProcessorConverter) InputComponentName() string {
	return "otelcol.processor.discovery"
}

func (discoveryProcessorConverter) ConvertAndAppend(state *otelcolconvert.State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	label := state.AlloyComponentLabel()

	args, diags := toDiscoveryProcessor(state, id, cfg.(*promsdprocessor.Config), label)
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "discovery"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", otelcolconvert.StringifyInstanceID(id), otelcolconvert.StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toDiscoveryProcessor(state *otelcolconvert.State, id componentstatus.InstanceID, cfg *promsdprocessor.Config, label string) (*otelcol_discovery.Arguments, diag.Diagnostics) {
	var (
		diags       diag.Diagnostics
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	// We need to Marshal/Unmarshal the scrape configs to translate them
	// into their actual types for the conversion.
	out, err := yaml.Marshal(cfg.ScrapeConfigs)
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("unable to marshal scrapeConfigs interface{} to yaml: %s", err))
		return nil, diags
	}
	scrapeConfigs := make([]*prom_config.ScrapeConfig, 0)
	err = yaml.Unmarshal(out, &scrapeConfigs)
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("unable to unmarshal bytes to []*config.ScrapeConfig: %s", err))
		return nil, diags
	}

	// Append the prometheus blocks to the file. prom_sd_processor makes use of
	// only the ServiceDiscoveryConfigs and RelabelConfigs from its ScrapeConfigs.
	// Other fields are ignored which is poorly designed Static mode config structure
	// but correct for the conversion.
	targets := []discovery.Target{}
	pb := build.NewPrometheusBlocks()
	for _, scrapeConfig := range scrapeConfigs {
		labelConcat := scrapeConfig.JobName
		if label != "" {
			labelConcat = label + "_" + scrapeConfig.JobName
		}
		label := common.SanitizeIdentifierPanics(labelConcat)
		scrapeTargets := prometheusconvert.AppendServiceDiscoveryConfigs(pb, scrapeConfig.ServiceDiscoveryConfigs, label)
		promDiscoveryRelabelExports := prometheus_component.AppendDiscoveryRelabel(pb, scrapeConfig.RelabelConfigs, scrapeTargets, label)
		if promDiscoveryRelabelExports != nil {
			scrapeTargets = promDiscoveryRelabelExports.Output
		}
		targets = append(targets, scrapeTargets...)
	}
	pb.AppendToBody(state.Body())

	return &otelcol_discovery.Arguments{
		Targets:         targets,
		OperationType:   cfg.OperationType,
		PodAssociations: cfg.PodAssociations,
		Output: &otelcol.ConsumerArguments{
			Metrics: otelcolconvert.ToTokenizedConsumers(nextMetrics),
			Logs:    otelcolconvert.ToTokenizedConsumers(nextLogs),
			Traces:  otelcolconvert.ToTokenizedConsumers(nextTraces),
		},
	}, diags
}
