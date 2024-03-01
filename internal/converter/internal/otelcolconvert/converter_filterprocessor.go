package otelcolconvert

import (
	"fmt"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor/filter"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, filterProcessorConverter{})
}

type filterProcessorConverter struct{}

func (filterProcessorConverter) Factory() component.Factory {
	return filterprocessor.NewFactory()
}

func (filterProcessorConverter) InputComponentName() string {
	return "otelcol.processor.filter"
}

func (filterProcessorConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toFilterProcessor(state, id, cfg.(*filterprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "filter"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toFilterProcessor(state *state, id component.InstanceID, cfg *filterprocessor.Config) *filter.Arguments {
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
		nextLogs    = state.Next(id, component.DataTypeLogs)
		nextTraces  = state.Next(id, component.DataTypeTraces)
	)

	return &filter.Arguments{
		ErrorMode: cfg.ErrorMode,
		Traces: filter.TraceConfig{
			Span:      cfg.Traces.SpanConditions,
			SpanEvent: cfg.Traces.SpanEventConditions,
		},
		Metrics: filter.MetricConfig{
			Metric:    cfg.Metrics.MetricConditions,
			Datapoint: cfg.Metrics.DataPointConditions,
		},
		Logs: filter.LogConfig{
			LogRecord: cfg.Logs.LogConditions,
		},
		Output: &otelcol.ConsumerArguments{
			Metrics: toTokenizedConsumers(nextMetrics),
			Logs:    toTokenizedConsumers(nextLogs),
			Traces:  toTokenizedConsumers(nextTraces),
		},
	}
}
