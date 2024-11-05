package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/opencensus"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, opencensusReceiverConverter{})
}

type opencensusReceiverConverter struct{}

func (opencensusReceiverConverter) Factory() component.Factory {
	return opencensusreceiver.NewFactory()
}

func (opencensusReceiverConverter) InputComponentName() string { return "" }

func (opencensusReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOpencensusReceiver(state, id, cfg.(*opencensusreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "opencensus"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOpencensusReceiver(state *State, id componentstatus.InstanceID, cfg *opencensusreceiver.Config) *opencensus.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &opencensus.Arguments{
		CorsAllowedOrigins: cfg.CorsOrigins,
		GRPC:               *toGRPCServerArguments(&cfg.ServerConfig),

		DebugMetrics: common.DefaultValue[opencensus.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
	}
}
