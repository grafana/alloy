//go:build !freebsd && !openbsd

package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/datadog"
	datadog_config "github.com/grafana/alloy/internal/component/otelcol/exporter/datadog/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	datadogOtelconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
)

func init() {
	converters = append(converters, datadogExporterConverter{})
}

type datadogExporterConverter struct{}

func (datadogExporterConverter) Factory() component.Factory { return datadogexporter.NewFactory() }

func (datadogExporterConverter) InputComponentName() string {
	return "otelcol.exporter.datadog"
}

func (datadogExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val any) any {
		switch val.(type) {
		case extension.ExtensionHandler:
			queue := cfg.(*datadogOtelconfig.Config).QueueSettings.GetOrInsertDefault()
			ext := state.LookupExtension(*queue.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toDatadogExporter(cfg.(*datadogOtelconfig.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "datadog"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toDatadogExporter(cfg *datadogOtelconfig.Config) *datadog.Arguments {
	return &datadog.Arguments{
		Client:                   toDatadogHTTPClientArguments(cfg.ClientConfig),
		Retry:                    toRetryArguments(cfg.BackOffConfig),
		Queue:                    toQueueArguments(cfg.QueueSettings),
		APISettings:              toDatadogAPIArguments(cfg.API),
		Traces:                   toDatadogTracesArguments(cfg.Traces),
		Metrics:                  toDatadogMetricsArguments(cfg.Metrics),
		Logs:                     toDatadogLogsArguments(cfg.Logs),
		HostMetadata:             toDatadogHostMetadataArguments(cfg.HostMetadata),
		OnlyMetadata:             cfg.OnlyMetadata,
		HostnameDetectionTimeout: cfg.HostnameDetectionTimeout,
		Hostname:                 cfg.Hostname,
		DebugMetrics:             common.DefaultValue[datadog.Arguments]().DebugMetrics,
	}
}

func toDatadogHTTPClientArguments(cfg confighttp.ClientConfig) datadog_config.DatadogClientArguments {
	return datadog_config.DatadogClientArguments{
		Timeout:             cfg.Timeout,
		ReadBufferSize:      cfg.ReadBufferSize,
		WriteBufferSize:     cfg.WriteBufferSize,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		DisableKeepAlives:   cfg.DisableKeepAlives,
		InsecureSkipVerify:  cfg.TLS.Insecure,
	}
}

func toDatadogLogsArguments(cfg datadogOtelconfig.LogsConfig) datadog_config.DatadogLogsArguments {
	return datadog_config.DatadogLogsArguments{
		Endpoint:         cfg.TCPAddrConfig.Endpoint,
		UseCompression:   cfg.UseCompression,
		CompressionLevel: cfg.CompressionLevel,
		BatchWait:        cfg.BatchWait,
	}
}

func toDatadogAPIArguments(cfg datadogOtelconfig.APIConfig) datadog_config.DatadogAPIArguments {
	return datadog_config.DatadogAPIArguments{
		Key:              alloytypes.Secret(cfg.Key),
		Site:             cfg.Site,
		FailOnInvalidKey: cfg.FailOnInvalidKey,
	}
}

func toDatadogTracesArguments(cfg datadogOtelconfig.TracesExporterConfig) datadog_config.DatadogTracesArguments {
	return datadog_config.DatadogTracesArguments{
		Endpoint:                  cfg.TCPAddrConfig.Endpoint,
		IgnoreResources:           cfg.IgnoreResources,
		SpanNameRemappings:        cfg.SpanNameRemappings,
		SpanNameAsResourceName:    cfg.SpanNameAsResourceName,
		ComputeStatsBySpanKind:    cfg.ComputeStatsBySpanKind,
		ComputeTopLevelBySpanKind: cfg.ComputeTopLevelBySpanKind,
		PeerTagsAggregation:       cfg.PeerTagsAggregation,
		PeerTags:                  cfg.PeerTags,
		TraceBuffer:               cfg.TraceBuffer,
	}
}

func toDatadogMetricsArguments(cfg datadogOtelconfig.MetricsConfig) datadog_config.DatadogMetricsArguments {
	return datadog_config.DatadogMetricsArguments{
		Endpoint:       cfg.TCPAddrConfig.Endpoint,
		DeltaTTL:       cfg.DeltaTTL,
		ExporterConfig: toDatadogExporterConfigArguments(cfg.ExporterConfig),
		HistConfig:     toDatadogHistogramArguments(cfg.HistConfig),
		SumConfig:      toDatadogSumArguments(cfg.SumConfig),
		SummaryConfig:  toDatadogSummaryArguments(cfg.SummaryConfig),
	}
}

func toDatadogExporterConfigArguments(cfg datadogOtelconfig.MetricsExporterConfig) datadog_config.DatadogMetricsExporterArguments {
	return datadog_config.DatadogMetricsExporterArguments{
		ResourceAttributesAsTags:           cfg.ResourceAttributesAsTags,
		InstrumentationScopeMetadataAsTags: cfg.InstrumentationScopeMetadataAsTags,
	}
}

func toDatadogHistogramArguments(cfg datadogOtelconfig.HistogramConfig) datadog_config.DatadogHistogramArguments {
	return datadog_config.DatadogHistogramArguments{
		SendAggregations: cfg.SendAggregations,
		Mode:             string(cfg.Mode),
	}
}

func toDatadogSumArguments(cfg datadogOtelconfig.SumConfig) datadog_config.DatadogSumArguments {
	return datadog_config.DatadogSumArguments{
		CumulativeMonotonicMode:        string(cfg.CumulativeMonotonicMode),
		InitialCumulativeMonotonicMode: string(cfg.InitialCumulativeMonotonicMode),
	}
}

func toDatadogSummaryArguments(cfg datadogOtelconfig.SummaryConfig) datadog_config.DatadogSummaryArguments {
	return datadog_config.DatadogSummaryArguments{
		Mode: string(cfg.Mode),
	}
}

func toDatadogHostMetadataArguments(cfg datadogOtelconfig.HostMetadataConfig) datadog_config.DatadogHostMetadataArguments {
	return datadog_config.DatadogHostMetadataArguments{
		Enabled:        cfg.Enabled,
		HostnameSource: string(cfg.HostnameSource),
		Tags:           cfg.Tags,
	}
}
