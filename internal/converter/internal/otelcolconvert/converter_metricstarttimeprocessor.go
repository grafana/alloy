package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/metricstarttime"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, metricStartTimeProcessorConverter{})
}

type metricStartTimeProcessorConverter struct{}

func (metricStartTimeProcessorConverter) Factory() component.Factory {
	return metricstarttimeprocessor.NewFactory()
}

func (metricStartTimeProcessorConverter) InputComponentName() string {
	return "otelcol.processor.metric_start_time"
}

func (metricStartTimeProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toMetricStartTimeProcessor(state, id, cfg.(*metricstarttimeprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "metric_start_time"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toMetricStartTimeProcessor(state *State, id componentstatus.InstanceID, cfg *metricstarttimeprocessor.Config) *metricstarttime.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &metricstarttime.Arguments{
		Strategy:             cfg.Strategy,
		GCInterval:           cfg.GCInterval,
		StartTimeMetricRegex: cfg.StartTimeMetricRegex,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[metricstarttime.Arguments]().DebugMetrics,
	}
}
