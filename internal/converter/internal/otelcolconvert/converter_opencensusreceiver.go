package otelcolconvert

import (
	"fmt"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver/opencensus"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, opencensusReceiverConverter{})
}

type opencensusReceiverConverter struct{}

func (opencensusReceiverConverter) Factory() component.Factory {
	return opencensusreceiver.NewFactory()
}

func (opencensusReceiverConverter) InputComponentName() string { return "" }

func (opencensusReceiverConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toOpencensusReceiver(state, id, cfg.(*opencensusreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "opencensus"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOpencensusReceiver(state *state, id component.InstanceID, cfg *opencensusreceiver.Config) *opencensus.Arguments {
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
		nextTraces  = state.Next(id, component.DataTypeTraces)
	)

	return &opencensus.Arguments{
		CorsAllowedOrigins: cfg.CorsOrigins,
		GRPC:               *toGRPCServerArguments(&cfg.GRPCServerSettings),

		DebugMetrics: common.DefaultValue[opencensus.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: toTokenizedConsumers(nextMetrics),
			Traces:  toTokenizedConsumers(nextTraces),
		},
	}
}
