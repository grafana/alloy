package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec"
	splunkhec_config "github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
)

func init() {
	converters = append(converters, splunkhecExporterConverter{})
}

type splunkhecExporterConverter struct{}

func (splunkhecExporterConverter) Factory() component.Factory { return splunkhecexporter.NewFactory() }

func (splunkhecExporterConverter) InputComponentName() string {
	return "otelcol.exporter.splunkhec"
}

func (splunkhecExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toSplunkHecExporter(cfg.(*splunkhecexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "splunkhec"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toSplunkHecExporter(cfg *splunkhecexporter.Config) *splunkhec.Arguments {
	return &splunkhec.Arguments{
		Client:       toSplunkHecHTTPClientArguments(cfg),
		Retry:        toRetryArguments(cfg.BackOffConfig),
		Queue:        toQueueArguments(cfg.QueueSettings),
		Splunk:       toSplunkConfig(cfg),
		DebugMetrics: common.DefaultValue[splunkhec.Arguments]().DebugMetrics,
	}
}

func toSplunkHecHTTPClientArguments(cfg *splunkhecexporter.Config) splunkhec_config.SplunkHecClientArguments {
	return splunkhec_config.SplunkHecClientArguments{
		Endpoint:            cfg.Endpoint,
		Timeout:             cfg.Timeout,
		ReadBufferSize:      cfg.ReadBufferSize,
		WriteBufferSize:     cfg.WriteBufferSize,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		DisableKeepAlives:   cfg.DisableKeepAlives,
		InsecureSkipVerify:  cfg.TLSSetting.Insecure,
	}
}

func toSplunkConfig(cfg *splunkhecexporter.Config) splunkhec_config.SplunkConf {
	return splunkhec_config.SplunkConf{
		Token:                   alloytypes.Secret(cfg.Token.String()),
		Source:                  cfg.Source,
		SourceType:              cfg.SourceType,
		Index:                   cfg.Index,
		LogDataEnabled:          cfg.LogDataEnabled,
		ProfilingDataEnabled:    cfg.ProfilingDataEnabled,
		DisableCompression:      cfg.DisableCompression,
		MaxContentLengthLogs:    cfg.MaxContentLengthLogs,
		MaxContentLengthMetrics: cfg.MaxContentLengthMetrics,
		MaxContentLengthTraces:  cfg.MaxContentLengthTraces,
		MaxEventSize:            cfg.MaxEventSize,
		SplunkAppName:           cfg.SplunkAppName,
		SplunkAppVersion:        cfg.SplunkAppVersion,
		HealthPath:              cfg.HealthPath,
		HecHealthCheckEnabled:   cfg.HecHealthCheckEnabled,
		ExportRaw:               cfg.ExportRaw,
		UseMultiMetricFormat:    cfg.UseMultiMetricFormat,
		Heartbeat:               toSplunkHecHeartbeat(cfg.Heartbeat),
		Telemetry:               toSplunkHecTelemetry(cfg.Telemetry),
		BatcherConfig:           toSplunkHecBatcherConfig(cfg.BatcherConfig),
		HecFields:               toSplunkHecFields(cfg.HecFields),
	}
}

func toSplunkHecHeartbeat(cfg splunkhecexporter.HecHeartbeat) splunkhec_config.SplunkHecHeartbeat {
	return splunkhec_config.SplunkHecHeartbeat{
		Interval: cfg.Interval,
		Startup:  cfg.Startup,
	}
}

func toSplunkHecTelemetry(cfg splunkhecexporter.HecTelemetry) splunkhec_config.SplunkHecTelemetry {
	return splunkhec_config.SplunkHecTelemetry{
		Enabled:              cfg.Enabled,
		OverrideMetricsNames: cfg.OverrideMetricsNames,
		ExtraAttributes:      cfg.ExtraAttributes,
	}
}

func toSplunkHecBatcherConfig(cfg exporterbatcher.Config) splunkhec_config.BatcherConfig {
	return splunkhec_config.BatcherConfig{
		Enabled:      cfg.Enabled,
		FlushTimeout: cfg.FlushTimeout,
		MinSizeItems: cfg.MinSizeItems,
		MaxSizeItems: cfg.MaxSizeItems,
	}
}

func toSplunkHecFields(cfg splunkhecexporter.OtelToHecFields) splunkhec_config.HecFields {
	return splunkhec_config.HecFields{
		SeverityText:   cfg.SeverityText,
		SeverityNumber: cfg.SeverityNumber,
	}
}
