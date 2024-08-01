package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/probabilistic_sampler"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, probabilisticSamplerProcessorConverter{})
}

type probabilisticSamplerProcessorConverter struct{}

func (probabilisticSamplerProcessorConverter) Factory() component.Factory {
	return probabilisticsamplerprocessor.NewFactory()
}

func (probabilisticSamplerProcessorConverter) InputComponentName() string {
	return "otelcol.processor.probabilistic_sampler"
}

func (probabilisticSamplerProcessorConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toProbabilisticSamplerProcessor(state, id, cfg.(*probabilisticsamplerprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "probabilistic_sampler"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toProbabilisticSamplerProcessor(state *State, id component.InstanceID, cfg *probabilisticsamplerprocessor.Config) *probabilistic_sampler.Arguments {
	var (
		nextTraces = state.Next(id, component.DataTypeTraces)
		nextLogs   = state.Next(id, component.DataTypeLogs)
	)

	return &probabilistic_sampler.Arguments{
		SamplingPercentage: cfg.SamplingPercentage,
		HashSeed:           cfg.HashSeed,
		Mode:               string(cfg.Mode),
		FailClosed:         cfg.FailClosed,
		SamplingPrecision:  cfg.SamplingPrecision,
		AttributeSource:    string(cfg.AttributeSource),
		FromAttribute:      cfg.FromAttribute,
		SamplingPriority:   cfg.SamplingPriority,
		Output: &otelcol.ConsumerArguments{
			Logs:   ToTokenizedConsumers(nextLogs),
			Traces: ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[probabilistic_sampler.Arguments]().DebugMetrics,
	}
}
