package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelconfig "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/googlecloudpubsub"
)

func init() {
	converters = append(converters, googleCloudPubSubReceiverConverter{})
}

type googleCloudPubSubReceiverConverter struct{}

func (googleCloudPubSubReceiverConverter) Factory() component.Factory {
	return googlecloudpubsubreceiver.NewFactory()
}

func (googleCloudPubSubReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.googlecloudpubsub"
}

func (c googleCloudPubSubReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	block := common.NewBlockWithOverride(
		strings.Split(c.InputComponentName(), "."),
		state.AlloyComponentLabel(),
		toGoogleCloudPubSubReceiver(state, id, cfg.(*googlecloudpubsubreceiver.Config)),
	)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toGoogleCloudPubSubReceiver(state *State, id componentstatus.InstanceID, cfg *googlecloudpubsubreceiver.Config) *googlecloudpubsub.Arguments {
	return &googlecloudpubsub.Arguments{
		ProjectID:           cfg.ProjectID,
		UserAgent:           cfg.UserAgent,
		Endpoint:            cfg.Endpoint,
		Insecure:            cfg.Insecure,
		Subscription:        cfg.Subscription,
		Encoding:            cfg.Encoding,
		Compression:         cfg.Compression,
		IgnoreEncodingError: cfg.IgnoreEncodingError,
		ClientID:            cfg.ClientID,
		Timeout:             cfg.TimeoutSettings.Timeout,
		DebugMetrics:        common.DefaultValue[otelconfig.DebugMetricsArguments](),
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(state.Next(id, pipeline.SignalLogs)),
			Logs:    ToTokenizedConsumers(state.Next(id, pipeline.SignalMetrics)),
			Traces:  ToTokenizedConsumers(state.Next(id, pipeline.SignalTraces)),
		},
	}
}
