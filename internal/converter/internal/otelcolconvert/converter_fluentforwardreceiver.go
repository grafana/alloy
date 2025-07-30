package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/fluentforward"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, fluentforwardReceiverConverter{})
}

type fluentforwardReceiverConverter struct{}

func (fluentforwardReceiverConverter) Factory() component.Factory {
	return fluentforwardreceiver.NewFactory()
}

func (fluentforwardReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.fluentforward"
}

func (fluentforwardReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolReceiverfluentforward(cfg.(*fluentforwardreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "fluentforward"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceiverfluentforward(cfg *fluentforwardreceiver.Config) *fluentforward.Arguments {
	args := &fluentforward.Arguments{
		Endpoint:     cfg.ListenAddress,
		DebugMetrics: common.DefaultValue[fluentforward.Arguments]().DebugMetrics,
	}

	return args
}
