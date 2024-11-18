package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/groupbyattrs"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, groupByAttrsConverter{})
}

type groupByAttrsConverter struct{}

func (groupByAttrsConverter) Factory() component.Factory {
	return groupbyattrsprocessor.NewFactory()
}

func (groupByAttrsConverter) InputComponentName() string {
	return "otelcol.processor.groupbyattrs"
}

func (groupByAttrsConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toGroupByAttrsProcessor(state, id, cfg.(*groupbyattrsprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "groupbyattrs"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toGroupByAttrsProcessor(state *State, id componentstatus.InstanceID, cfg *groupbyattrsprocessor.Config) *groupbyattrs.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &groupbyattrs.Arguments{
		Keys: cfg.GroupByKeys,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},

		DebugMetrics: common.DefaultValue[groupbyattrs.Arguments]().DebugMetrics,
	}
}
