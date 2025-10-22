package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/filter"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
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

func (filterProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toFilterProcessor(state, id, cfg.(*filterprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "filter"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toFilterProcessor(state *State, id componentstatus.InstanceID, cfg *filterprocessor.Config) *filter.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
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
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[filter.Arguments]().DebugMetrics,
	}
}
