package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/interval"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, intervalProcessorConverter{})
}

type intervalProcessorConverter struct{}

func (intervalProcessorConverter) Factory() component.Factory {
	return intervalprocessor.NewFactory()
}

func (intervalProcessorConverter) InputComponentName() string {
	return "otelcol.processor.interval"
}

func (intervalProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toIntervalProcessor(state, id, cfg.(*intervalprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "interval"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toIntervalProcessor(state *State, id componentstatus.InstanceID, cfg *intervalprocessor.Config) *interval.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &interval.Arguments{
		Interval: cfg.Interval,
		PassThrough: interval.PassThrough{
			Gauge:   cfg.PassThrough.Gauge,
			Summary: cfg.PassThrough.Summary,
		},
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[interval.Arguments]().DebugMetrics,
	}
}
