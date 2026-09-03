// Package clickhouse provides an otelcol.exporter.clickhouse component.
package clickhouse

import (
	"errors"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
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
	// Endpoint is the ClickHouse DSN, e.g. "tcp://localhost:9000" or "clickhouse://localhost:9000".
	Endpoint         string            `alloy:"endpoint,attr"`
	Username         string            `alloy:"username,attr,optional"`
	Password         alloytypes.Secret `alloy:"password,attr,optional"`
	Database         string            `alloy:"database,attr,optional"`
	ConnectionParams map[string]string `alloy:"connection_params,attr,optional"`

	TLS otelcol.TLSClientArguments `alloy:"tls,block,optional"`

	LogsTableName    string `alloy:"logs_table_name,attr,optional"`
	TracesTableName  string `alloy:"traces_table_name,attr,optional"`
	MetricsTableName string `alloy:"metrics_table_name,attr,optional"`

	TTL          time.Duration        `alloy:"ttl,attr,optional"`
	TableEngine  TableEngineArguments `alloy:"table_engine,block,optional"`
	ClusterName  string               `alloy:"cluster_name,attr,optional"`
	CreateSchema bool                 `alloy:"create_schema,attr,optional"`
	Compress     string               `alloy:"compress,attr,optional"`
	AsyncInsert  bool                 `alloy:"async_insert,attr,optional"`

	MetricsTables MetricsTablesArguments `alloy:"metrics_tables,block,optional"`

	Timeout time.Duration          `alloy:"timeout,attr,optional"`
	Retry   otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`
	Queue   otelcol.QueueArguments `alloy:"sending_queue,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// TableEngineArguments configures the ClickHouse table ENGINE clause.
type TableEngineArguments struct {
	Name   string `alloy:"name,attr,optional"`
	Params string `alloy:"params,attr,optional"`
}

// MetricsTablesArguments configures per-metric-type table names.
type MetricsTablesArguments struct {
	Gauge                MetricTypeArguments `alloy:"gauge,block,optional"`
	Sum                  MetricTypeArguments `alloy:"sum,block,optional"`
	Summary              MetricTypeArguments `alloy:"summary,block,optional"`
	Histogram            MetricTypeArguments `alloy:"histogram,block,optional"`
	ExponentialHistogram MetricTypeArguments `alloy:"exponential_histogram,block,optional"`
}

// MetricTypeArguments configures the table name override for one metric type.
type MetricTypeArguments struct {
	Name string `alloy:"name,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Database:         "default",
		ConnectionParams: map[string]string{},
		LogsTableName:    "otel_logs",
		TracesTableName:  "otel_traces",
		MetricsTableName: "otel_metrics",
		TableEngine:      TableEngineArguments{Name: "MergeTree"},
		CreateSchema:     true,
		Compress:         "lz4",
		AsyncInsert:      true,
	}
	args.Retry.SetToDefault()
	args.Queue.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Endpoint == "" {
		return errors.New("endpoint must be specified")
	}
	return nil
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}

	cfg := &clickhouseexporter.Config{
		TimeoutSettings:  exporterhelper.TimeoutConfig{Timeout: args.Timeout},
		BackOffConfig:    *args.Retry.Convert(),
		QueueSettings:    q,
		Endpoint:         args.Endpoint,
		Username:         args.Username,
		Password:         configopaque.String(args.Password),
		Database:         args.Database,
		TLS:              *args.TLS.Convert(),
		ConnectionParams: args.ConnectionParams,
		LogsTableName:    args.LogsTableName,
		TracesTableName:  args.TracesTableName,
		MetricsTableName: args.MetricsTableName,
		TTL:              args.TTL,
		TableEngine: clickhouseexporter.TableEngine{
			Name:   args.TableEngine.Name,
			Params: args.TableEngine.Params,
		},
		ClusterName:  args.ClusterName,
		CreateSchema: args.CreateSchema,
		Compress:     args.Compress,
		AsyncInsert:  args.AsyncInsert,
	}
	// MetricTypeConfig lives in clickhouseexporter's internal/metrics package, so it
	// can't be named here (Go forbids importing another module's internal packages).
	// Set the nested fields through selectors instead of a composite literal.
	cfg.MetricsTables.Gauge.Name = args.MetricsTables.Gauge.Name
	cfg.MetricsTables.Sum.Name = args.MetricsTables.Sum.Name
	cfg.MetricsTables.Summary.Name = args.MetricsTables.Summary.Name
	cfg.MetricsTables.Histogram.Name = args.MetricsTables.Histogram.Name
	cfg.MetricsTables.ExponentialHistogram.Name = args.MetricsTables.ExponentialHistogram.Name
	return cfg, nil
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
