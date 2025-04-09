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

	args := toCumulativetodeltaProcessor(state, id, cfg.(*cumulativetodeltaprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "cumulativetodelta"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toCumulativetodeltaProcessor(state *State, id componentstatus.InstanceID, cfg *cumulativetodeltaprocessor.Config) *cumulativetodelta.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &cumulativetodelta.Arguments{
		MaxStaleness: cfg.MaxStaleness,
		InitialValue: cfg.InitialValue.String(),
		Include: cumulativetodelta.MatchArgs{
			Metrics:   cfg.Include.Metrics,
			MatchType: string(cfg.Include.MatchType),
		},
		Exclude: cumulativetodelta.MatchArgs{
			Metrics:   cfg.Exclude.Metrics,
			MatchType: string(cfg.Exclude.MatchType),
		},
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[cumulativetodelta.Arguments]().DebugMetrics,
	}
}
