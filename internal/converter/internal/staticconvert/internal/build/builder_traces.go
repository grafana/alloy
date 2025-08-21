package build

import (
	"fmt"
	"reflect"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/static/traces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	otel_component "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/otelcol"
	p "go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/service/pipelines"
)

// List of component converters. This slice is appended to by init functions in
// other files.
var converters []otelcolconvert.ComponentConverter

func (b *ConfigBuilder) appendTraces() {
	if reflect.DeepEqual(b.cfg.Traces, traces.Config{}) {
		return
	}

	for _, cfg := range b.cfg.Traces.Configs {
		otelCfg, err := cfg.OtelConfig()
		if err != nil {
			b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to load otelConfig from agent traces config: %s", err))
			continue
		}

		// Only prefix component labels if we are doing more than 1 trace config.
		labelPrefix := ""
		if len(b.cfg.Traces.Configs) > 1 {
			labelPrefix = cfg.Name
		}

		// Remove the push receiver which is an implementation detail for static mode and unnecessary for the otel config.
		removeReceiver(otelCfg, p.SignalTraces, otel_component.MustNewType("push_receiver"))

		// Remove the service_graphs processor which is an implementation detail for static mode and unnecessary for the otel config.
		if _, ok := otelCfg.Processors[otel_component.NewID(otel_component.MustNewType("service_graphs"))]; ok {
			removeProcessor(otelCfg, p.SignalTraces, otel_component.MustNewType("service_graphs"))
			b.diags.Add(diag.SeverityLevelError, "The service_graphs processor for traces has no direct Alloy equivalent. "+
				"This configuration appends metrics to the /metrics endpoint of the agent which is not possible in Alloy. "+
				"Alternatively, you can use the otelcol.connector.servicegraph component to build a pipeline which generates "+
				"and forwards service graph metrics.")
		}

		b.translateAutomaticLogging(otelCfg, cfg)
		b.translateSpanMetrics(otelCfg, cfg)

		b.diags.AddAll(otelcolconvert.AppendConfig(b.f, otelCfg, labelPrefix, converters, false))
	}
}

func (b *ConfigBuilder) translateAutomaticLogging(otelCfg *otelcol.Config, cfg traces.InstanceConfig) {
	if _, ok := otelCfg.Processors[otel_component.NewID(otel_component.MustNewType("automatic_logging"))]; !ok {
		return
	}

	if cfg.AutomaticLogging.Backend == "stdout" {
		b.diags.Add(diag.SeverityLevelWarn, "automatic_logging for traces has no direct Alloy equivalent. "+
			"A best effort translation has been made to otelcol.exporter.debug but the behavior will differ.")
	} else {
		b.diags.Add(diag.SeverityLevelError, "automatic_logging for traces has no direct Alloy equivalent. "+
			"A best effort translation can be made which only outputs to stdout and not directly to loki by bypassing errors.")
	}

	// Add the debug exporter to the otel config with default values
	otelCfg.Exporters[otel_component.NewID(otel_component.MustNewType("debug"))] = debugexporter.NewFactory().CreateDefaultConfig()

	// Add the debug exporter to all pipelines
	for _, pipeline := range otelCfg.Service.Pipelines {
		pipeline.Exporters = append(pipeline.Exporters, otel_component.NewID(otel_component.MustNewType("debug")))
	}

	// Remove the custom automatic_logging processor
	removeProcessor(otelCfg, p.SignalTraces, otel_component.MustNewType("automatic_logging"))
}

func (b *ConfigBuilder) translateSpanMetrics(otelCfg *otelcol.Config, cfg traces.InstanceConfig) {
	if _, ok := otelCfg.Processors[otel_component.NewID(otel_component.MustNewType("spanmetrics"))]; !ok {
		return
	}

	// Remove the custom otel components and delete the custom metrics pipeline
	removeProcessor(otelCfg, p.SignalTraces, otel_component.MustNewType("spanmetrics"))
	removeReceiver(otelCfg, p.SignalMetrics, otel_component.MustNewType("noop"))
	removeExporter(otelCfg, p.SignalMetrics, otel_component.MustNewType("prometheus"))
	removePipeline(otelCfg, p.SignalMetrics, "spanmetrics")

	// If the spanmetrics configuration includes a handler_endpoint, we cannot convert it.
	// This is intentionally after the section above which removes the custom spanmetrics processor
	// so that the rest of the configuration can optionally be converted with the error.
	if cfg.SpanMetrics.HandlerEndpoint != "" {
		b.diags.Add(diag.SeverityLevelError, "Cannot convert using configuration including spanmetrics handler_endpoint. "+
			"No equivalent exists for exposing a known /metrics endpoint. You can use metrics_instance instead to enabled conversion.")
		return
	}

	// Add the spanmetrics connector to the otel config with the converted configuration
	if otelCfg.Connectors == nil {
		otelCfg.Connectors = map[otel_component.ID]otel_component.Config{}
	}
	otelCfg.Connectors[otel_component.NewID(otel_component.MustNewType("spanmetrics"))] = toSpanmetricsConnector(cfg.SpanMetrics)

	// Add the spanmetrics connector to each traces pipelines as an exporter and create metrics pipelines.
	// The processing ordering for the span metrics connector differs from the static pipelines since tail sampling
	// in static mode processes after the custom span metrics processor. This is ok because the tail sampling
	// processor is not processing metrics.
	spanmetricsID := otel_component.NewID(otel_component.MustNewType("spanmetrics"))
	remoteWriteID := otel_component.NewID(otel_component.MustNewType("remote_write"))
	for ix, pipeline := range otelCfg.Service.Pipelines {
		if ix.Signal() == p.SignalTraces {
			pipeline.Exporters = append(pipeline.Exporters, spanmetricsID)

			metricsId := p.NewIDWithName(p.SignalMetrics, ix.Name())
			otelCfg.Service.Pipelines[metricsId] = &pipelines.PipelineConfig{}
			otelCfg.Service.Pipelines[metricsId].Receivers = append(otelCfg.Service.Pipelines[metricsId].Receivers, spanmetricsID)
			otelCfg.Service.Pipelines[metricsId].Exporters = append(otelCfg.Service.Pipelines[metricsId].Exporters, remoteWriteID)
		}
	}
}

func toSpanmetricsConnector(cfg *traces.SpanMetricsConfig) *spanmetricsconnector.Config {
	smc := spanmetricsconnector.NewFactory().CreateDefaultConfig().(*spanmetricsconnector.Config)
	for _, dim := range cfg.Dimensions {
		smc.Dimensions = append(smc.Dimensions, spanmetricsconnector.Dimension{Name: dim.Name, Default: dim.Default})
	}
	if cfg.DimensionsCacheSize != 0 {
		smc.DimensionsCacheSize = cfg.DimensionsCacheSize
	}
	if cfg.AggregationTemporality != "" {
		smc.AggregationTemporality = cfg.AggregationTemporality
	}
	if len(cfg.LatencyHistogramBuckets) != 0 {
		smc.Histogram.Explicit = configoptional.Some(spanmetricsconnector.ExplicitHistogramConfig{Buckets: cfg.LatencyHistogramBuckets})
	}
	if cfg.MetricsFlushInterval != 0 {
		smc.MetricsFlushInterval = cfg.MetricsFlushInterval
	}
	if cfg.Namespace != "" {
		smc.Namespace = cfg.Namespace
	}

	return smc
}

// removeReceiver removes a receiver from the otel config for a specific pipeline type.
func removeReceiver(otelCfg *otelcol.Config, pipelineType p.Signal, receiverType otel_component.Type) {
	if _, ok := otelCfg.Receivers[otel_component.NewID(receiverType)]; !ok {
		return
	}

	delete(otelCfg.Receivers, otel_component.NewID(receiverType))
	for ix, p := range otelCfg.Service.Pipelines {
		if ix.Signal() != pipelineType {
			continue
		}

		spr := make([]otel_component.ID, 0)
		for _, r := range p.Receivers {
			if r.Type() != receiverType {
				spr = append(spr, r)
			}
		}
		otelCfg.Service.Pipelines[ix].Receivers = spr
	}
}

// removeProcessor removes a processor from the otel config for a specific pipeline type.
func removeProcessor(otelCfg *otelcol.Config, pipelineType p.Signal, processorType otel_component.Type) {
	if _, ok := otelCfg.Processors[otel_component.NewID(processorType)]; !ok {
		return
	}

	delete(otelCfg.Processors, otel_component.NewID(processorType))
	for ix, p := range otelCfg.Service.Pipelines {
		if ix.Signal() != pipelineType {
			continue
		}

		spr := make([]otel_component.ID, 0)
		for _, r := range p.Processors {
			if r.Type() != processorType {
				spr = append(spr, r)
			}
		}
		otelCfg.Service.Pipelines[ix].Processors = spr
	}
}

// removeExporter removes an exporter from the otel config for a specific pipeline type.
func removeExporter(otelCfg *otelcol.Config, pipelineType p.Signal, exporterType otel_component.Type) {
	if _, ok := otelCfg.Exporters[otel_component.NewID(exporterType)]; !ok {
		return
	}

	delete(otelCfg.Exporters, otel_component.NewID(exporterType))
	for ix, p := range otelCfg.Service.Pipelines {
		if ix.Signal() != pipelineType {
			continue
		}

		spr := make([]otel_component.ID, 0)
		for _, r := range p.Exporters {
			if r.Type() != exporterType {
				spr = append(spr, r)
			}
		}
		otelCfg.Service.Pipelines[ix].Exporters = spr
	}
}

// removePipeline removes a pipeline from the otel config for a specific pipeline type.
func removePipeline(otelCfg *otelcol.Config, pipelineType p.Signal, pipelineName string) {
	delete(otelCfg.Service.Pipelines, p.NewIDWithName(pipelineType, pipelineName))
}
