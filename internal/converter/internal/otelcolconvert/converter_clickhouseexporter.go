package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/clickhouse"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	clickhouseexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configtls"
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
	overrideHook := func(val any) any {
		switch val.(type) {
		case extension.ExtensionHandler:
			queue := cfg.(*clickhouseexporter.Config).QueueSettings.GetOrInsertDefault()
			ext := state.LookupExtension(*queue.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toClickHouseExporter(cfg.(*clickhouseexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "clickhouse"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toClickHouseExporter(cfg *clickhouseexporter.Config) *clickhouse.Arguments {
	args := &clickhouse.Arguments{
		Endpoint:         cfg.Endpoint,
		Username:         cfg.Username,
		Password:         alloytypes.Secret(string(cfg.Password)),
		Database:         cfg.Database,
		ConnectionParams: cfg.ConnectionParams,
		LogsTableName:    cfg.LogsTableName,
		TracesTableName:  cfg.TracesTableName,
		TTL:              cfg.TTL,
		ClusterName:      cfg.ClusterName,
		CreateSchema:     cfg.CreateSchema,
		Compress:         cfg.Compress,
		AsyncInsert:      cfg.AsyncInsert,
		Timeout:          cfg.TimeoutSettings.Timeout,
		JSON:             cfg.JSON,
		MetricsTables:    toClickHouseMetricsTables(cfg.MetricsTables),
		Queue:            toQueueArguments(cfg.QueueSettings),
		Retry:            toRetryArguments(cfg.BackOffConfig),
		DebugMetrics:     common.DefaultValue[clickhouse.Arguments]().DebugMetrics,
	}

	if cfg.TableEngine.Name != "" {
		args.TableEngine = &clickhouse.TableEngineArguments{
			Name:   cfg.TableEngine.Name,
			Params: cfg.TableEngine.Params,
		}
	}

	if tls := toClickHouseTLS(cfg.TLS); tls != nil {
		args.TLS = tls
	}

	return args
}

func toClickHouseMetricsTables(cfg clickhouseexporter.MetricTablesConfig) *clickhouse.MetricTablesArguments {
	return &clickhouse.MetricTablesArguments{
		Gauge:                toClickHouseMetricTable(cfg.Gauge.Name),
		Sum:                  toClickHouseMetricTable(cfg.Sum.Name),
		Summary:              toClickHouseMetricTable(cfg.Summary.Name),
		Histogram:            toClickHouseMetricTable(cfg.Histogram.Name),
		ExponentialHistogram: toClickHouseMetricTable(cfg.ExponentialHistogram.Name),
	}
}

func toClickHouseMetricTable(name string) *clickhouse.MetricTableConfig {
	if name == "" {
		return nil
	}
	return &clickhouse.MetricTableConfig{Name: name}
}

func toClickHouseTLS(cfg configtls.ClientConfig) *otelcol.TLSClientArguments {
	if cfg.CAFile == "" && cfg.CertFile == "" && cfg.KeyFile == "" &&
		cfg.ServerName == "" && !cfg.Insecure && !cfg.InsecureSkipVerify {
		return nil
	}
	tls := toTLSClientArguments(cfg)
	return &tls
}
