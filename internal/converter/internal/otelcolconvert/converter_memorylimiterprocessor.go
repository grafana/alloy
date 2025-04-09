package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/memorylimiter"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
)

func init() {
	converters = append(converters, memoryLimiterProcessorConverter{})
}

type memoryLimiterProcessorConverter struct{}

func (memoryLimiterProcessorConverter) Factory() component.Factory {
	return memorylimiterprocessor.NewFactory()
}

func (memoryLimiterProcessorConverter) InputComponentName() string {
	return "otelcol.processor.memory_limiter"
}
func (memoryLimiterProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toMemoryLimiterProcessor(state, id, cfg.(*memorylimiterprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "memory_limiter"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)

	return diags
}

func toMemoryLimiterProcessor(state *State, id componentstatus.InstanceID, cfg *memorylimiterprocessor.Config) *memorylimiter.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &memorylimiter.Arguments{
		CheckInterval:         cfg.CheckInterval,
		MemoryLimit:           units.Base2Bytes(cfg.MemoryLimitMiB) * units.MiB,
		MemorySpikeLimit:      units.Base2Bytes(cfg.MemorySpikeLimitMiB) * units.MiB,
		MemoryLimitPercentage: cfg.MemoryLimitPercentage,
		MemorySpikePercentage: cfg.MemorySpikePercentage,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[memorylimiter.Arguments]().DebugMetrics,
	}
}
