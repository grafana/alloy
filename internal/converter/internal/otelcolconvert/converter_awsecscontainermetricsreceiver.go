package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/awsecscontainermetrics"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, awsEcsContainerMetricsReceiverConverter{})
}

type awsEcsContainerMetricsReceiverConverter struct{}

func (awsEcsContainerMetricsReceiverConverter) Factory() component.Factory {
	return awsecscontainermetricsreceiver.NewFactory()
}

func (awsEcsContainerMetricsReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.awsecscontainermetrics"
}

func (awsEcsContainerMetricsReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toAwsEcsContainerMetricsReceiver(state, id, cfg.(*awsecscontainermetricsreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "awsecscontainermetrics"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAwsEcsContainerMetricsReceiver(state *State, id componentstatus.InstanceID, cfg *awsecscontainermetricsreceiver.Config) *awsecscontainermetrics.Arguments {
	nextMetrics := state.Next(id, pipeline.SignalMetrics)

	return &awsecscontainermetrics.Arguments{
		CollectionInterval: cfg.CollectionInterval,
		DebugMetrics:       common.DefaultValue[awsecscontainermetrics.Arguments]().DebugMetrics,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},
	}
}
