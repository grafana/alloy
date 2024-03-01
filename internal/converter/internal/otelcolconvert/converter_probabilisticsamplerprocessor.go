package otelcolconvert

import (
	"fmt"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor/probabilistic_sampler"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
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

func (probabilisticSamplerProcessorConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toProbabilisticSamplerProcessor(state, id, cfg.(*probabilisticsamplerprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "probabilistic_sampler"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toProbabilisticSamplerProcessor(state *state, id component.InstanceID, cfg *probabilisticsamplerprocessor.Config) *probabilistic_sampler.Arguments {
	var (
		nextTraces = state.Next(id, component.DataTypeTraces)
		nextLogs   = state.Next(id, component.DataTypeLogs)
	)

	return &probabilistic_sampler.Arguments{
		SamplingPercentage: cfg.SamplingPercentage,
		HashSeed:           cfg.HashSeed,
		AttributeSource:    string(cfg.AttributeSource),
		FromAttribute:      cfg.FromAttribute,
		SamplingPriority:   cfg.SamplingPriority,
		Output: &otelcol.ConsumerArguments{
			Logs:   toTokenizedConsumers(nextLogs),
			Traces: toTokenizedConsumers(nextTraces),
		},
	}
}
