package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/transform"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, transformProcessorConverter{})
}

type transformProcessorConverter struct{}

func (transformProcessorConverter) Factory() component.Factory {
	return transformprocessor.NewFactory()
}

func (transformProcessorConverter) InputComponentName() string {
	return "otelcol.processor.transform"
}

func (transformProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toTransformProcessor(state, id, cfg.(*transformprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "transform"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toTransformProcessor(state *State, id componentstatus.InstanceID, cfg *transformprocessor.Config) *transform.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &transform.Arguments{
		ErrorMode:        cfg.ErrorMode,
		TraceStatements:  toContextStatements(encodeMapslice(cfg.TraceStatements)),
		MetricStatements: toContextStatements(encodeMapslice(cfg.MetricStatements)),
		LogStatements:    toContextStatements(encodeMapslice(cfg.LogStatements)),
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[transform.Arguments]().DebugMetrics,
	}
}

func toContextStatements(in []map[string]any) []transform.ContextStatements {
	res := make([]transform.ContextStatements, 0, len(in))
	for _, s := range in {
		res = append(res, transform.ContextStatements{
			Context:    transform.ContextID(encodeString(s["context"])),
			Statements: s["statements"].([]string),
			Conditions: s["conditions"].([]string),
			ErrorMode:  s["error_mode"].(ottl.ErrorMode),
		})
	}

	return res
}
