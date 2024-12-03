package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/batchprocessor"
)

func init() {
	converters = append(converters, batchProcessorConverter{})
}

type batchProcessorConverter struct{}

func (batchProcessorConverter) Factory() component.Factory {
	return batchprocessor.NewFactory()
}

func (batchProcessorConverter) InputComponentName() string {
	return "otelcol.processor.batch"
}

func (batchProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toBatchProcessor(state, id, cfg.(*batchprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "batch"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toBatchProcessor(state *State, id componentstatus.InstanceID, cfg *batchprocessor.Config) *batch.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &batch.Arguments{
		Timeout:                  cfg.Timeout,
		SendBatchSize:            cfg.SendBatchSize,
		SendBatchMaxSize:         cfg.SendBatchMaxSize,
		MetadataKeys:             cfg.MetadataKeys,
		MetadataCardinalityLimit: cfg.MetadataCardinalityLimit,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[batch.Arguments]().DebugMetrics,
	}
}
