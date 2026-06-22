package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/opensearch/config"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, opensearchExporterConverter{})
}

type opensearchExporterConverter struct{}

func (opensearchExporterConverter) Factory() component.Factory {
	return opensearchexporter.NewFactory()
}

func (opensearchExporterConverter) InputComponentName() string {
	return "otelcol.exporter.opensearch"
}

func (opensearchExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	args := toOpenSearchExporter(cfg.(*opensearchexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "opensearch"}, label, args, common.GetAlloyTypesOverrideHook())

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOpenSearchExporter(cfg *opensearchexporter.Config) *config.OpenSearchArguments {
	return &config.OpenSearchArguments{
		Dataset:               cfg.Dataset,
		Namespace:             cfg.Namespace,
		LogsIndex:             cfg.LogsIndex,
		LogsIndexFallback:     cfg.LogsIndexFallback,
		LogsIndexTimeFormat:   cfg.LogsIndexTimeFormat,
		TracesIndex:           cfg.TracesIndex,
		TracesIndexFallback:   cfg.TracesIndexFallback,
		TracesIndexTimeFormat: cfg.TracesIndexTimeFormat,
		BulkAction:            cfg.BulkAction,
		Timeout:               cfg.TimeoutSettings.Timeout,

		Client: toHTTPClientArguments(cfg.ClientConfig),
		Mapping: config.MappingArguments{
			Mode:           cfg.MappingsSettings.Mode,
			Fields:         cfg.MappingsSettings.Fields,
			File:           cfg.MappingsSettings.File,
			TimestampField: cfg.MappingsSettings.TimestampField,
			UnixTimestamp:  cfg.MappingsSettings.UnixTimestamp,
			Dedup:          cfg.MappingsSettings.Dedup,
			Dedot:          cfg.MappingsSettings.Dedot,
		},

		SendingQueue: toQueueArguments(cfg.QueueConfig),
		Retry:        toRetryArguments(cfg.BackOffConfig),
		DebugMetrics: common.DefaultValue[config.OpenSearchArguments]().DebugMetrics,
	}
}
