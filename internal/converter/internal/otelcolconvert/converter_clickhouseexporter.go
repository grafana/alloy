package otelcolconvert

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/clickhouse"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	clickhouseexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
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
	defaultCfg := clickhouseexporter.NewFactory().CreateDefaultConfig().(*clickhouseexporter.Config)

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
		MetricsTables:    toClickHouseMetricsTables(cfg.MetricsTables, defaultCfg.MetricsTables),
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

	if !reflect.DeepEqual(cfg.TLS, defaultCfg.TLS) {
		tls := toTLSClientArguments(cfg.TLS)
		args.TLS = &tls
	}

	return args
}

// toClickHouseMetricsTables only keeps the table names which differ from the
// upstream defaults, so that default values aren't written to the output.
func toClickHouseMetricsTables(cfg, defaults clickhouseexporter.MetricTablesConfig) *clickhouse.MetricTablesArguments {
	args := &clickhouse.MetricTablesArguments{
		Gauge:                toClickHouseMetricTable(cfg.Gauge.Name, defaults.Gauge.Name),
		Sum:                  toClickHouseMetricTable(cfg.Sum.Name, defaults.Sum.Name),
		Summary:              toClickHouseMetricTable(cfg.Summary.Name, defaults.Summary.Name),
		Histogram:            toClickHouseMetricTable(cfg.Histogram.Name, defaults.Histogram.Name),
		ExponentialHistogram: toClickHouseMetricTable(cfg.ExponentialHistogram.Name, defaults.ExponentialHistogram.Name),
	}
	if *args == (clickhouse.MetricTablesArguments{}) {
		return nil
	}
	return args
}

func toClickHouseMetricTable(name, defaultName string) *clickhouse.MetricTableConfig {
	if name == "" || name == defaultName {
		return nil
	}
	return &clickhouse.MetricTableConfig{Name: name}
}
