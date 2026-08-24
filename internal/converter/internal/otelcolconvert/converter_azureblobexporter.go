package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/azureblob"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, azureBlobExporterConverter{})
}

type azureBlobExporterConverter struct{}

func (azureBlobExporterConverter) Factory() component.Factory {
	return azureblobexporter.NewFactory()
}

func (azureBlobExporterConverter) InputComponentName() string {
	return "otelcol.exporter.azureblob"
}

func (azureBlobExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	azureBlobCfg := cfg.(*azureblobexporter.Config)
	args := toAzureBlobExporter(azureBlobCfg)
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "azureblob"}, label, args)

	if azureBlobCfg.Encodings.Logs != nil || azureBlobCfg.Encodings.Metrics != nil || azureBlobCfg.Encodings.Traces != nil {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("%s: encodings are not supported and were ignored", StringifyInstanceID(id)),
		)
	}
	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAzureBlobExporter(cfg *azureblobexporter.Config) *azureblob.Arguments {
	return &azureblob.Arguments{
		Timeout: cfg.TimeoutSettings.Timeout,
		Queue:   toQueueArguments(cfg.QueueSettings),
		Retry:   toRetryArguments(cfg.BackOffConfig),
		URL:     cfg.URL,
		Format:  cfg.FormatType,
		Auth:    toAzureBlobAuth(cfg.Auth),
		Container: azureblob.TelemetryContainer{
			Logs:    cfg.Container.Logs,
			Metrics: cfg.Container.Metrics,
			Traces:  cfg.Container.Traces,
		},
		BlobNameFormat: toAzureBlobNameFormat(cfg.BlobNameFormat),
		AppendBlob: azureblob.AppendBlob{
			Enabled:   cfg.AppendBlob.Enabled,
			Separator: cfg.AppendBlob.Separator,
		},
		DebugMetrics: common.DefaultValue[azureblob.Arguments]().DebugMetrics,
	}
}

func toAzureBlobAuth(cfg azureblobexporter.Authentication) azureblob.Authentication {
	return azureblob.Authentication{
		Type:               string(cfg.Type),
		TenantID:           cfg.TenantID,
		ClientID:           cfg.ClientID,
		ClientSecret:       cfg.ClientSecret,
		ConnectionString:   cfg.ConnectionString,
		FederatedTokenFile: cfg.FederatedTokenFile,
	}
}

func toAzureBlobNameFormat(cfg azureblobexporter.BlobNameFormat) azureblob.BlobNameFormat {
	return azureblob.BlobNameFormat{
		MetricsFormat:            cfg.MetricsFormat,
		LogsFormat:               cfg.LogsFormat,
		TracesFormat:             cfg.TracesFormat,
		SerialNumEnabled:         cfg.SerialNumEnabled,
		SerialNumRange:           cfg.SerialNumRange,
		SerialNumBeforeExtension: cfg.SerialNumBeforeExtension,
		Timezone:                 cfg.Timezone,
		TemplateEnabled:          cfg.TemplateEnabled,
		TimeParserEnabled:        cfg.TimeParserEnabled,
		TimeParserRanges:         cfg.TimeParserRanges,
		Params:                   cfg.Params,
	}
}
