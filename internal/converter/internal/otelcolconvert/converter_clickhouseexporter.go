package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/clickhouse"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, clickhouseExporterConverter{})
}

type clickhouseExporterConverter struct{}

func (clickhouseExporterConverter) Factory() component.Factory {
	return clickhouseexporter.NewFactory()
}

func (clickhouseExporterConverter) InputComponentName() string {
	return "otelcol.exporter.clickhouse"
}

func (clickhouseExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolExporterClickhouse(cfg.(*clickhouseexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "clickhouse"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolExporterClickhouse(cfg *clickhouseexporter.Config) *clickhouse.Arguments {
	return &clickhouse.Arguments{
		Endpoint:         cfg.Endpoint,
		Username:         cfg.Username,
		Password:         alloytypes.Secret(cfg.Password),
		Database:         cfg.Database,
		ConnectionParams: cfg.ConnectionParams,
		TLS:              toTLSClientArguments(cfg.TLS),
		LogsTableName:    cfg.LogsTableName,
		TracesTableName:  cfg.TracesTableName,
		// MetricsTableName is deprecated upstream in favor of MetricsTables
		// (converted below), which is what cfg actually resolves to by the
		// time Convert() runs on a validated config.
		TTL: cfg.TTL,
		TableEngine: clickhouse.TableEngineArguments{
			Name:   cfg.TableEngine.Name,
			Params: cfg.TableEngine.Params,
		},
		ClusterName:  cfg.ClusterName,
		CreateSchema: cfg.CreateSchema,
		Compress:     cfg.Compress,
		AsyncInsert:  cfg.AsyncInsert,
		MetricsTables: clickhouse.MetricsTablesArguments{
			Gauge:                clickhouse.MetricTypeArguments{Name: cfg.MetricsTables.Gauge.Name},
			Sum:                  clickhouse.MetricTypeArguments{Name: cfg.MetricsTables.Sum.Name},
			Summary:              clickhouse.MetricTypeArguments{Name: cfg.MetricsTables.Summary.Name},
			Histogram:            clickhouse.MetricTypeArguments{Name: cfg.MetricsTables.Histogram.Name},
			ExponentialHistogram: clickhouse.MetricTypeArguments{Name: cfg.MetricsTables.ExponentialHistogram.Name},
		},
		Timeout:      cfg.TimeoutSettings.Timeout,
		Retry:        toRetryArguments(cfg.BackOffConfig),
		Queue:        toQueueArguments(cfg.QueueSettings),
		DebugMetrics: common.DefaultValue[clickhouse.Arguments]().DebugMetrics,
	}
}
