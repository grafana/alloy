package otelcolconvert

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/cloudflare"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func init() {
	converters = append(converters, cloudflareReceiverConverter{})
}

type cloudflareReceiverConverter struct{}

func (cloudflareReceiverConverter) Factory() component.Factory {
	return cloudflarereceiver.NewFactory()
}

func (cloudflareReceiverConverter) InputComponentName() string { return "" }

func (cloudflareReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toCloudflareReceiver(state, id, cfg.(*cloudflarereceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "cloudflare"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toCloudflareReceiver(state *State, id componentstatus.InstanceID, cfg *cloudflarereceiver.Config) *cloudflare.Arguments {
	nextLogs := state.Next(id, pipeline.SignalLogs)

	return &cloudflare.Arguments{
		Endpoint:        cfg.Logs.Endpoint,
		Secret:          cfg.Logs.Secret,
		TLS:             toTLSServerArguments(cfg.Logs.TLS),
		Attributes:      cfg.Logs.Attributes,
		TimestampField:  cfg.Logs.TimestampField,
		TimestampFormat: cfg.Logs.TimestampFormat,
		Separator:       cfg.Logs.Separator,
		Output: &otelcol.ConsumerArguments{
			Logs: ToTokenizedConsumers(nextLogs),
		},
	}
}
