package otelcolconvert

import (
	"fmt"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor/batch"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
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

func (batchProcessorConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toBatchProcessor(state, id, cfg.(*batchprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "batch"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toBatchProcessor(state *state, id component.InstanceID, cfg *batchprocessor.Config) *batch.Arguments {
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
		nextLogs    = state.Next(id, component.DataTypeLogs)
		nextTraces  = state.Next(id, component.DataTypeTraces)
	)

	return &batch.Arguments{
		Timeout:                  cfg.Timeout,
		SendBatchSize:            cfg.SendBatchSize,
		SendBatchMaxSize:         cfg.SendBatchMaxSize,
		MetadataKeys:             cfg.MetadataKeys,
		MetadataCardinalityLimit: cfg.MetadataCardinalityLimit,
		Output: &otelcol.ConsumerArguments{
			Metrics: toTokenizedConsumers(nextMetrics),
			Logs:    toTokenizedConsumers(nextLogs),
			Traces:  toTokenizedConsumers(nextTraces),
		},
	}
}
