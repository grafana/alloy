package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/nginx"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, nginxReceiverConverter{})
}

type nginxReceiverConverter struct{}

func (nginxReceiverConverter) Factory() component.Factory {
	return nginxreceiver.NewFactory()
}

func (nginxReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.nginx"
}

func (nginxReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toNginxReceiver(state, id, cfg.(*nginxreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "nginx"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toNginxReceiver(state *State, id componentstatus.InstanceID, cfg *nginxreceiver.Config) *nginx.Arguments {
	nextMetrics := state.Next(id, pipeline.SignalMetrics)

	return &nginx.Arguments{
		Endpoint:           cfg.Endpoint,
		CollectionInterval: cfg.CollectionInterval,
		InitialDelay:       cfg.InitialDelay,
		DebugMetrics:       common.DefaultValue[nginx.Arguments]().DebugMetrics,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},
	}
}
