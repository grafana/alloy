package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/zipkin"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, zipkinReceiverConverter{})
}

type zipkinReceiverConverter struct{}

func (zipkinReceiverConverter) Factory() component.Factory { return zipkinreceiver.NewFactory() }

func (zipkinReceiverConverter) InputComponentName() string { return "" }

func (zipkinReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toZipkinReceiver(state, id, cfg.(*zipkinreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "zipkin"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toZipkinReceiver(state *State, id componentstatus.InstanceID, cfg *zipkinreceiver.Config) *zipkin.Arguments {
	var (
		nextTraces = state.Next(id, pipeline.SignalTraces)
	)

	return &zipkin.Arguments{
		ParseStringTags: cfg.ParseStringTags,
		HTTPServer:      *toHTTPServerArguments(&cfg.ServerConfig),

		DebugMetrics: common.DefaultValue[zipkin.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Traces: ToTokenizedConsumers(nextTraces),
		},
	}
}
