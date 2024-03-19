package build

import (
	"fmt"
	"reflect"

	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/otelcolconvert"
	"github.com/grafana/agent/internal/static/traces"
	otel_component "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/otelcol"
)

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

		// Remove the push receiver which is an implementation detail for static mode and unnecessary for the otel config.
		removeReceiver(otelCfg, "traces", "push_receiver")

		b.translateAutomaticLogging(otelCfg, cfg)

		// Only prefix component labels if we are doing more than 1 trace config.
		labelPrefix := ""
		if len(b.cfg.Traces.Configs) > 1 {
			labelPrefix = cfg.Name
		}
		b.diags.AddAll(otelcolconvert.AppendConfig(b.f, otelCfg, labelPrefix))
	}
}

func (b *ConfigBuilder) translateAutomaticLogging(otelCfg *otelcol.Config, cfg traces.InstanceConfig) {
	if _, ok := otelCfg.Processors[otel_component.NewID("automatic_logging")]; !ok {
		return
	}

	if cfg.AutomaticLogging.Backend == "stdout" {
		b.diags.Add(diag.SeverityLevelWarn, "automatic_logging for traces has no direct flow equivalent. "+
			"A best effort translation has been made to otelcol.exporter.logging but the behavior will differ.")
	} else {
		b.diags.Add(diag.SeverityLevelError, "automatic_logging for traces has no direct flow equivalent. "+
			"A best effort translation can be made which only outputs to stdout and not directly to loki by bypassing errors.")
	}

	// Add the logging exporter to the otel config with default values
	otelCfg.Exporters[otel_component.NewID("logging")] = loggingexporter.NewFactory().CreateDefaultConfig()

	// Add the logging exporter to all pipelines
	for _, pipeline := range otelCfg.Service.Pipelines {
		pipeline.Exporters = append(pipeline.Exporters, otel_component.NewID("logging"))
	}

	// Remove the custom automatic_logging processor
	removeProcessor(otelCfg, "traces", "automatic_logging")
}

// removeReceiver removes a receiver from the otel config for a specific pipeline type.
func removeReceiver(otelCfg *otelcol.Config, pipelineType otel_component.Type, receiverType otel_component.Type) {
	if _, ok := otelCfg.Receivers[otel_component.NewID(receiverType)]; !ok {
		return
	}

	delete(otelCfg.Receivers, otel_component.NewID(receiverType))
	spr := make([]otel_component.ID, 0, len(otelCfg.Service.Pipelines[otel_component.NewID(pipelineType)].Receivers)-1)
	for _, r := range otelCfg.Service.Pipelines[otel_component.NewID(pipelineType)].Receivers {
		if r != otel_component.NewID(receiverType) {
			spr = append(spr, r)
		}
	}
	otelCfg.Service.Pipelines[otel_component.NewID(pipelineType)].Receivers = spr
}

// removeProcessor removes a processor from the otel config for a specific pipeline type.
func removeProcessor(otelCfg *otelcol.Config, pipelineType otel_component.Type, processorType otel_component.Type) {
	if _, ok := otelCfg.Processors[otel_component.NewID(processorType)]; !ok {
		return
	}

	delete(otelCfg.Processors, otel_component.NewID(processorType))
	spr := make([]otel_component.ID, 0, len(otelCfg.Service.Pipelines[otel_component.NewID(pipelineType)].Processors)-1)
	for _, r := range otelCfg.Service.Pipelines[otel_component.NewID(pipelineType)].Processors {
		if r != otel_component.NewID(processorType) {
			spr = append(spr, r)
		}
	}
	otelCfg.Service.Pipelines[otel_component.NewID(pipelineType)].Processors = spr
}
