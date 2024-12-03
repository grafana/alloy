package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/datadog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, datadogReceiverConverter{})
}

type datadogReceiverConverter struct{}

func (datadogReceiverConverter) Factory() component.Factory { return datadogreceiver.NewFactory() }

func (datadogReceiverConverter) InputComponentName() string { return "" }

func (datadogReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toDatadogReceiver(state, id, cfg.(*datadogreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "datadog"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toDatadogReceiver(state *State, id componentstatus.InstanceID, cfg *datadogreceiver.Config) *datadog.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &datadog.Arguments{
		HTTPServer:  *toHTTPServerArguments(&cfg.ServerConfig),
		ReadTimeout: cfg.ReadTimeout,

		DebugMetrics: common.DefaultValue[datadog.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
	}
}
