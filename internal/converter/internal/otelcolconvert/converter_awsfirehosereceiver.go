package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/awsfirehose"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, awsFirehoseReceiverConverter{})
}

type awsFirehoseReceiverConverter struct{}

func (awsFirehoseReceiverConverter) Factory() component.Factory {
	return awsfirehosereceiver.NewFactory()
}

func (awsFirehoseReceiverConverter) InputComponentName() string { return "" }

func (awsFirehoseReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toAWSFirehoseReceiver(state, id, cfg.(*awsfirehosereceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "awsfirehose"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAWSFirehoseReceiver(state *State, id componentstatus.InstanceID, cfg *awsfirehosereceiver.Config) *awsfirehose.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
	)

	return &awsfirehose.Arguments{
		Encoding:  cfg.Encoding,
		AccessKey: alloytypes.Secret(cfg.AccessKey),

		HTTPServer: *toHTTPServerArguments(&cfg.ServerConfig),

		DebugMetrics: common.DefaultValue[awsfirehose.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
		},
	}
}
