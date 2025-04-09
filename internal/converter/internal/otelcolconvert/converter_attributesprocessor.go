package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/attributes"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, attributesProcessorConverter{})
}

type attributesProcessorConverter struct{}

func (attributesProcessorConverter) Factory() component.Factory {
	return attributesprocessor.NewFactory()
}

func (attributesProcessorConverter) InputComponentName() string {
	return "otelcol.processor.attributes"
}

func (attributesProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toAttributesProcessor(state, id, cfg.(*attributesprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "attributes"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAttributesProcessor(state *State, id componentstatus.InstanceID, cfg *attributesprocessor.Config) *attributes.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
	)

	return &attributes.Arguments{
		Match:   toMatchConfig(cfg),
		Actions: toAttrActionKeyValue(encodeMapslice(cfg.Actions)),
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces)},
		DebugMetrics: common.DefaultValue[attributes.Arguments]().DebugMetrics,
	}
}

func toMatchConfig(cfg *attributesprocessor.Config) otelcol.MatchConfig {
	return otelcol.MatchConfig{
		Include: toMatchProperties(encodeMapstruct(cfg.Include)),
		Exclude: toMatchProperties(encodeMapstruct(cfg.Exclude)),
	}
}

func toAttrActionKeyValue(cfg []map[string]any) []otelcol.AttrActionKeyValue {
	result := make([]otelcol.AttrActionKeyValue, 0)

	for _, action := range cfg {
		result = append(result, otelcol.AttrActionKeyValue{
			Key:           action["key"].(string),
			Value:         action["value"],
			RegexPattern:  action["pattern"].(string),
			FromAttribute: action["from_attribute"].(string),
			FromContext:   action["from_context"].(string),
			ConvertedType: action["converted_type"].(string),
			Action:        encodeString(action["action"]),
		})
	}

	return result
}
