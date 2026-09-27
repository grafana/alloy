// Package clickhouse provides an otelcol.exporter.clickhouse component.
package clickhouse

import (
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	clickhouseexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	otelpexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.clickhouse",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := clickhouseexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

// Arguments configures the otelcol.exporter.clickhouse component.
type Arguments struct {
	Endpoint         string            `alloy:"endpoint,attr"`
	Username         string            `alloy:"username,attr,optional"`
	Password         alloytypes.Secret `alloy:"password,attr,optional"`
	Database         string            `alloy:"database,attr,optional"`
	ConnectionParams map[string]string `alloy:"connection_params,attr,optional"`

	LogsTableName   string `alloy:"logs_table_name,attr,optional"`
	TracesTableName string `alloy:"traces_table_name,attr,optional"`

	TTL          time.Duration `alloy:"ttl,attr,optional"`
	ClusterName  string        `alloy:"cluster_name,attr,optional"`
	CreateSchema bool          `alloy:"create_schema,attr,optional"`
	Compress     string        `alloy:"compress,attr,optional"`
	AsyncInsert  bool          `alloy:"async_insert,attr,optional"`
	Timeout      time.Duration `alloy:"timeout,attr,optional"`
	JSON         bool          `alloy:"json,attr,optional"`

	TableEngine   *TableEngineArguments       `alloy:"table_engine,block,optional"`
	MetricsTables *MetricTablesArguments      `alloy:"metrics_tables,block,optional"`
	TLS           *otelcol.TLSClientArguments `alloy:"tls,block,optional"`

	Queue        otelcol.QueueArguments           `alloy:"sending_queue,block,optional"`
	Retry        otelcol.RetryArguments           `alloy:"retry_on_failure,block,optional"`
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// TableEngineArguments configures the ClickHouse table engine.
type TableEngineArguments struct {
	Name   string `alloy:"name,attr,optional"`
	Params string `alloy:"params,attr,optional"`
}

// MetricTablesArguments configures the table names for each metric type.
type MetricTablesArguments struct {
	Gauge                *MetricTableConfig `alloy:"gauge,block,optional"`
	Sum                  *MetricTableConfig `alloy:"sum,block,optional"`
	Summary              *MetricTableConfig `alloy:"summary,block,optional"`
	Histogram            *MetricTableConfig `alloy:"histogram,block,optional"`
	ExponentialHistogram *MetricTableConfig `alloy:"exponential_histogram,block,optional"`
}

// MetricTableConfig configures a single metric type table name.
type MetricTableConfig struct {
	Name string `alloy:"name,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Database:        "default",
		LogsTableName:   "otel_logs",
		TracesTableName: "otel_traces",
		CreateSchema:    true,
		Compress:        "lz4",
		AsyncInsert:     true,
		Timeout:         otelcol.DefaultTimeout,
	}
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	cfg, err := args.Convert()
	if err != nil {
		return err
	}
	return cfg.(*clickhouseexporter.Config).Validate()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}

	metricsTables, err := args.convertMetricsTables()
	if err != nil {
		return nil, err
	}

	cfg := &clickhouseexporter.Config{
		TimeoutSettings: otelpexporterhelper.TimeoutConfig{
			Timeout: args.Timeout,
		},
		BackOffConfig:    *args.Retry.Convert(),
		QueueSettings:    q,
		Endpoint:         args.Endpoint,
		Username:         args.Username,
		Password:         configopaque.String(args.Password),
		Database:         args.Database,
		ConnectionParams: args.ConnectionParams,
		LogsTableName:   args.LogsTableName,
		TracesTableName: args.TracesTableName,
		TTL:             args.TTL,
		ClusterName:      args.ClusterName,
		CreateSchema:     args.CreateSchema,
		Compress:         args.Compress,
		AsyncInsert:      args.AsyncInsert,
		JSON:             args.JSON,
		MetricsTables:    metricsTables,
	}

	if args.TableEngine != nil {
		cfg.TableEngine = clickhouseexporter.TableEngine{
			Name:   args.TableEngine.Name,
			Params: args.TableEngine.Params,
		}
	}

	if args.TLS != nil {
		tlsCfg := args.TLS.Convert()
		if tlsCfg != nil {
			cfg.TLS = *tlsCfg
		}
	}

	return cfg, nil
}

func (args Arguments) convertMetricsTables() (clickhouseexporter.MetricTablesConfig, error) {
	// Use the upstream factory defaults as the base, then override with user-provided values.
	fact := clickhouseexporter.NewFactory()
	defaultCfg := fact.CreateDefaultConfig().(*clickhouseexporter.Config)
	result := defaultCfg.MetricsTables

	if args.MetricsTables == nil {
		return result, nil
	}

	mt := args.MetricsTables
	input := map[string]any{}

	if mt.Gauge != nil && mt.Gauge.Name != "" {
		input["gauge"] = map[string]any{"name": mt.Gauge.Name}
	}
	if mt.Sum != nil && mt.Sum.Name != "" {
		input["sum"] = map[string]any{"name": mt.Sum.Name}
	}
	if mt.Summary != nil && mt.Summary.Name != "" {
		input["summary"] = map[string]any{"name": mt.Summary.Name}
	}
	if mt.Histogram != nil && mt.Histogram.Name != "" {
		input["histogram"] = map[string]any{"name": mt.Histogram.Name}
	}
	if mt.ExponentialHistogram != nil && mt.ExponentialHistogram.Name != "" {
		input["exponential_histogram"] = map[string]any{"name": mt.ExponentialHistogram.Name}
	}

	if len(input) > 0 {
		if err := mapstructure.Decode(input, &result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.Queue.Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
