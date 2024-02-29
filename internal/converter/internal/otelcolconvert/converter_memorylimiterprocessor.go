package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor/memorylimiter"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
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
func (memoryLimiterProcessorConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toMemoryLimiterProcessor(state, id, cfg.(*memorylimiterprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "memory_limiter"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)

	return diags
}

func toMemoryLimiterProcessor(state *state, id component.InstanceID, cfg *memorylimiterprocessor.Config) *memorylimiter.Arguments {
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
		nextLogs    = state.Next(id, component.DataTypeLogs)
		nextTraces  = state.Next(id, component.DataTypeTraces)
	)

	return &memorylimiter.Arguments{
		CheckInterval:         cfg.CheckInterval,
		MemoryLimit:           units.Base2Bytes(cfg.MemoryLimitMiB) * units.MiB,
		MemorySpikeLimit:      units.Base2Bytes(cfg.MemorySpikeLimitMiB) * units.MiB,
		MemoryLimitPercentage: cfg.MemoryLimitPercentage,
		MemorySpikePercentage: cfg.MemorySpikePercentage,
		Output: &otelcol.ConsumerArguments{
			Metrics: toTokenizedConsumers(nextMetrics),
			Logs:    toTokenizedConsumers(nextLogs),
			Traces:  toTokenizedConsumers(nextTraces),
		},
	}
}
