package otelcolconvert

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/connector/exceptions"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func init() {
	converters = append(converters, exceptionsConnectorConverter{})
}

type exceptionsConnectorConverter struct{}

func (exceptionsConnectorConverter) Factory() component.Factory {
	return exceptionsconnector.NewFactory()
}

func (exceptionsConnectorConverter) InputComponentName() string {
	return "otelcol.connector.exceptions"
}

func (exceptionsConnectorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toExceptionsConnector(state, id, cfg.(*exceptionsconnector.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "connector", "exceptions"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toExceptionsConnector(state *State, id componentstatus.InstanceID, cfg *exceptionsconnector.Config) *exceptions.Arguments {
	if cfg == nil {
		return nil
	}
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
	)

	dimensions := make([]exceptions.Dimension, 0, len(cfg.Dimensions))
	for _, d := range cfg.Dimensions {
		dimensions = append(dimensions, exceptions.Dimension{
			Name:    d.Name,
			Default: d.Default,
		})
	}

	return &exceptions.Arguments{
		Dimensions: dimensions,
		Exemplars: exceptions.ExemplarsConfig{
			Enabled: cfg.Exemplars.Enabled,
		},
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
		},
		DebugMetrics: common.DefaultValue[exceptions.Arguments]().DebugMetrics,
	}
}
