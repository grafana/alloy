package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"

	otelconfig "github.com/grafana/alloy/internal/component/otelcol/config"
	googlecloudpubsubconfig "github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloudpubsub/config"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloudpubsub"
)

func init() {
	converters = append(converters, googleCloudPubSubExporterConverter{})
}

type googleCloudPubSubExporterConverter struct{}

func (googleCloudPubSubExporterConverter) Factory() component.Factory {
	return googlecloudpubsubexporter.NewFactory()
}

func (googleCloudPubSubExporterConverter) InputComponentName() string {
	return "otelcol.exporter.googlecloudpubsub"
}

func (c googleCloudPubSubExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	block := common.NewBlockWithOverride(
		strings.Split(c.InputComponentName(), "."),
		state.AlloyComponentLabel(),
		toGoogleCloudPubSubExporter(cfg.(*googlecloudpubsubexporter.Config)),
	)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toGoogleCloudPubSubExporter(cfg *googlecloudpubsubexporter.Config) *googlecloudpubsub.Arguments {
	return &googlecloudpubsub.Arguments{
		Retry:       toRetryArguments(cfg.BackOffConfig),
		Queue:       toQueueArguments(cfg.QueueSettings),
		Project:     cfg.ProjectID,
		UserAgent:   cfg.UserAgent,
		Topic:       cfg.Topic,
		Compression: cfg.Compression,
		Watermark: googlecloudpubsubconfig.GoogleCloudPubSubWatermarkArguments{
			Behavior:     cfg.Watermark.Behavior,
			AllowedDrift: cfg.Watermark.AllowedDrift,
		},
		Endpoint: cfg.Endpoint,
		Insecure: cfg.Insecure,
		Ordering: googlecloudpubsubconfig.GoogleCloudPubSubOrderingConfigArguments{
			Enabled:                 cfg.Ordering.Enabled,
			FromResourceAttribute:   cfg.Ordering.FromResourceAttribute,
			RemoveResourceAttribute: cfg.Ordering.RemoveResourceAttribute,
		},
		Timeout: cfg.TimeoutSettings.Timeout,

		DebugMetrics: common.DefaultValue[otelconfig.DebugMetricsArguments](),
	}
}
