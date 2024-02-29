package otelcolconvert

import (
	"fmt"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver/zipkin"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, zipkinReceiverConverter{})
}

type zipkinReceiverConverter struct{}

func (zipkinReceiverConverter) Factory() component.Factory { return zipkinreceiver.NewFactory() }

func (zipkinReceiverConverter) InputComponentName() string { return "" }

func (zipkinReceiverConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toZipkinReceiver(state, id, cfg.(*zipkinreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "zipkin"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toZipkinReceiver(state *state, id component.InstanceID, cfg *zipkinreceiver.Config) *zipkin.Arguments {
	var (
		nextTraces = state.Next(id, component.DataTypeTraces)
	)

	return &zipkin.Arguments{
		ParseStringTags: cfg.ParseStringTags,
		HTTPServer:      *toHTTPServerArguments(&cfg.HTTPServerSettings),

		DebugMetrics: common.DefaultValue[zipkin.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Traces: toTokenizedConsumers(nextTraces),
		},
	}
}
