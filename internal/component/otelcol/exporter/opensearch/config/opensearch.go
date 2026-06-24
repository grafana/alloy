// Package config contains configuration arguments for the
// otelcol.exporter.opensearch component.
package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

const (
	BulkActionCreate = "create"
	BulkActionIndex  = "index"

	MappingSS4O              = "ss4o"
	MappingECS               = "ecs"
	MappingFlattenAttributes = "flatten_attributes"
	MappingBodyMap           = "bodymap"
)

// OpenSearchArguments configures the otelcol.exporter.opensearch component.
type OpenSearchArguments struct {
	Dataset   string `alloy:"dataset,attr,optional"`
	Namespace string `alloy:"namespace,attr,optional"`

	LogsIndex           string `alloy:"logs_index,attr,optional"`
	LogsIndexFallback   string `alloy:"logs_index_fallback,attr,optional"`
	LogsIndexTimeFormat string `alloy:"logs_index_time_format,attr,optional"`

	TracesIndex           string `alloy:"traces_index,attr,optional"`
	TracesIndexFallback   string `alloy:"traces_index_fallback,attr,optional"`
	TracesIndexTimeFormat string `alloy:"traces_index_time_format,attr,optional"`

	BulkAction string        `alloy:"bulk_action,attr,optional"`
	Timeout    time.Duration `alloy:"timeout,attr,optional"`

	Client  otelcol.HTTPClientArguments `alloy:"client,block"`
	Mapping MappingArguments            `alloy:"mapping,block,optional"`

	SendingQueue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry        otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// MappingArguments configures OpenSearch field-mapping behavior.
type MappingArguments struct {
	Mode           string            `alloy:"mode,attr,optional"`
	Fields         map[string]string `alloy:"fields,attr,optional"`
	File           string            `alloy:"file,attr,optional"`
	TimestampField string            `alloy:"timestamp_field,attr,optional"`
	UnixTimestamp  bool              `alloy:"unix_timestamp,attr,optional"`
	Dedup          bool              `alloy:"dedup,attr,optional"`
	Dedot          bool              `alloy:"dedot,attr,optional"`
}

var (
	_ syntax.Defaulter = (*OpenSearchArguments)(nil)
	_ syntax.Validator = (*OpenSearchArguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *OpenSearchArguments) SetToDefault() {
	*args = OpenSearchArguments{
		Dataset:    "default",
		Namespace:  "namespace",
		BulkAction: BulkActionCreate,
	}
	args.Mapping.SetToDefault()
	args.SendingQueue.SetToDefault()
	args.Retry.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// SetToDefault implements syntax.Defaulter.
func (args *MappingArguments) SetToDefault() {
	*args = MappingArguments{
		Mode: MappingSS4O,
	}
}

// Validate implements syntax.Validator.
func (args *OpenSearchArguments) Validate() error {
	if args.Client.Endpoint == "" {
		return errors.New("client endpoint must be specified")
	}
	if args.Dataset == "" {
		return errors.New("dataset must be specified")
	}
	if args.Namespace == "" {
		return errors.New("namespace must be specified")
	}
	if args.BulkAction != BulkActionCreate && args.BulkAction != BulkActionIndex {
		return fmt.Errorf("bulk_action must be %q or %q", BulkActionCreate, BulkActionIndex)
	}
	switch args.Mapping.Mode {
	case MappingSS4O, MappingECS, MappingFlattenAttributes, MappingBodyMap:
	default:
		return fmt.Errorf("mapping.mode must be one of %q, %q, %q, or %q",
			MappingSS4O, MappingECS, MappingFlattenAttributes, MappingBodyMap)
	}
	return nil
}

// Convert implements exporter.Arguments.
func (args OpenSearchArguments) Convert() (otelcomponent.Config, error) {
	clientCfg, err := args.Client.Convert()
	if err != nil {
		return nil, err
	}

	q, err := args.SendingQueue.Convert()
	if err != nil {
		return nil, err
	}

	return &opensearchexporter.Config{
		ClientConfig:  *clientCfg,
		BackOffConfig: *args.Retry.Convert(),
		TimeoutSettings: exporterhelper.TimeoutConfig{
			Timeout: args.Timeout,
		},
		MappingsSettings: opensearchexporter.MappingsSettings{
			Mode:           args.Mapping.Mode,
			Fields:         args.Mapping.Fields,
			File:           args.Mapping.File,
			TimestampField: args.Mapping.TimestampField,
			UnixTimestamp:  args.Mapping.UnixTimestamp,
			Dedup:          args.Mapping.Dedup,
			Dedot:          args.Mapping.Dedot,
		},
		QueueConfig:           q,
		Dataset:               args.Dataset,
		Namespace:             args.Namespace,
		LogsIndex:             args.LogsIndex,
		LogsIndexFallback:     args.LogsIndexFallback,
		LogsIndexTimeFormat:   args.LogsIndexTimeFormat,
		TracesIndex:           args.TracesIndex,
		TracesIndexFallback:   args.TracesIndexFallback,
		TracesIndexTimeFormat: args.TracesIndexTimeFormat,
		BulkAction:            args.BulkAction,
	}, nil
}

// Extensions implements exporter.Arguments.
func (args OpenSearchArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	for k, v := range args.Client.Extensions() {
		m[k] = v
	}
	for k, v := range args.SendingQueue.Extensions() {
		m[k] = v
	}
	return m
}

// Exporters implements exporter.Arguments.
func (args OpenSearchArguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args OpenSearchArguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
