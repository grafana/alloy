package otelcolconvert

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/bridge"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"

	// Register the bridged components so the loop below can find them.
	_ "github.com/grafana/alloy/internal/component/otelcol/processor/batch"
)

// Bridged components don't need a hand-written converter: their Alloy
// arguments wrap the upstream configuration directly.
func init() {
	for _, p := range bridge.Processors() {
		converters = append(converters, bridgeProcessorConverter{p})
	}
}

type bridgeProcessorConverter struct {
	processor bridge.Processor
}

func (c bridgeProcessorConverter) Factory() component.Factory {
	return c.processor.Factory
}

func (c bridgeProcessorConverter) InputComponentName() string {
	return c.processor.Name
}

func (c bridgeProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	output := &otelcol.ConsumerArguments{
		Metrics: ToTokenizedConsumers(state.Next(id, pipeline.SignalMetrics)),
		Logs:    ToTokenizedConsumers(state.Next(id, pipeline.SignalLogs)),
		Traces:  ToTokenizedConsumers(state.Next(id, pipeline.SignalTraces)),
	}
	args := c.processor.NewArguments(cfg, output)
	block := common.NewBlockWithOverride(strings.Split(c.processor.Name, "."), label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}
