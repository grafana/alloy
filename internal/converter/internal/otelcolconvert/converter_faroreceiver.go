package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/faro"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, faroReceiverConverter{})
}

type faroReceiverConverter struct{}

func (faroReceiverConverter) Factory() component.Factory {
	return faroreceiver.NewFactory()
}

func (faroReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.faro"
}

func (faroReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolReceiverFaro(state, id, cfg.(*faroreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "faro"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceiverFaro(state *State, id componentstatus.InstanceID, cfg *faroreceiver.Config) *faro.Arguments {
	var (
		nextLogs   = state.Next(id, pipeline.SignalLogs)
		nextTraces = state.Next(id, pipeline.SignalTraces)
	)

	return &faro.Arguments{
		HTTPServer: *toHTTPServerArguments(&cfg.ServerConfig),

		DebugMetrics: common.DefaultValue[faro.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Logs:   ToTokenizedConsumers(nextLogs),
			Traces: ToTokenizedConsumers(nextTraces),
		},
	}
}
