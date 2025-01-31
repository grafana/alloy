package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/cumulativetodelta"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, cumulativetodeltaProcessorConverter{})
}

type cumulativetodeltaProcessorConverter struct{}

func (cumulativetodeltaProcessorConverter) Factory() component.Factory {
	return cumulativetodeltaprocessor.NewFactory()
}

func (cumulativetodeltaProcessorConverter) InputComponentName() string {
	return "otelcol.processor.cumulativetodelta"
}

func (cumulativetodeltaProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := tocumulativetodeltaProcessor(state, id, cfg.(*cumulativetodeltaprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "cumulativetodelta"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func tocumulativetodeltaProcessor(state *State, id componentstatus.InstanceID, cfg *cumulativetodeltaprocessor.Config) *cumulativetodelta.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &cumulativetodelta.Arguments{
		MaxStaleness: cfg.MaxStaleness,
		InitialValue: cumulativetodelta.InitialValue(cfg.InitialValue),
		Include:      toMatchMetrics(cfg.Include),
		Exclude:      toMatchMetrics(cfg.Exclude),
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[cumulativetodelta.Arguments]().DebugMetrics,
	}
}

func toMatchMetrics(mm cumulativetodeltaprocessor.MatchMetrics) *cumulativetodelta.MatchMetrics {
	return &cumulativetodelta.MatchMetrics{
		MatchType: string(mm.MatchType),
		Metrics:   mm.Metrics,
		// MetricTypes: mm.MetricTypes, // TODO uncomment in v0.118
	}
}
