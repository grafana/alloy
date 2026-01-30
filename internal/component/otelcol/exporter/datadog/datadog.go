// Package datadog provides an otelcol.exporter.datadog component.
// Maintainers for the Grafana Alloy wrapper:
//	- @polyrain

//go:build !freebsd && !openbsd

package datadog

import (
	"errors"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	datadog_config "github.com/grafana/alloy/internal/component/otelcol/exporter/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	datadogOtelconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pipeline"
)

const (
	DATADOG_TRACE_ENDPOINT   = "https://trace.agent.%s"
	DATADOG_METRICS_ENDPOINT = "https://api.%s"
	DATADOG_LOGS_ENDPOINT    = "https://http-intake.logs.%s"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.datadog",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := datadogexporter.NewFactory()
			// The Exporter skips APM stat computation by default, suggesting to use the Connector to do this.
			// Since we don't have that, we disable the feature gate to allow the exporter to compute APM stats.
			// See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/datadogexporter for more
			_ = featuregate.GlobalRegistry().Set("exporter.datadogexporter.DisableAPMStats", false)
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

// Arguments configures the otelcol.exporter.datadog component.
type Arguments struct {
	Client datadog_config.DatadogClientArguments `alloy:"client,block,optional"`
	Queue  otelcol.QueueArguments                `alloy:"sending_queue,block,optional"`
	Retry  otelcol.RetryArguments                `alloy:"retry_on_failure,block,optional"`

	// Datadog specific configuration settings
	APISettings              datadog_config.DatadogAPIArguments          `alloy:"api,block"`
	Traces                   datadog_config.DatadogTracesArguments       `alloy:"traces,block,optional"`
	Metrics                  datadog_config.DatadogMetricsArguments      `alloy:"metrics,block,optional"`
	Logs                     datadog_config.DatadogLogsArguments         `alloy:"logs,block,optional"`
	HostMetadata             datadog_config.DatadogHostMetadataArguments `alloy:"host_metadata,block,optional"`
	HostnameDetectionTimeout time.Duration                               `alloy:"hostname_detection_timeout,attr,optional"`
	OnlyMetadata             bool                                        `alloy:"only_metadata,attr,optional"`
	Hostname                 string                                      `alloy:"hostname,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// DatadogAPISettings holds the configuration settings for the Datadog API.

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Client.SetToDefault()
	args.APISettings.SetToDefault()
	args.Metrics.SetToDefault()
	args.Traces.SetToDefault()
	args.Logs.SetToDefault()
	args.HostMetadata.SetToDefault()
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.DebugMetrics.SetToDefault()
	args.HostnameDetectionTimeout = 25 * time.Second
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	// Prepare default endpoints for traces and metrics based on the site value
	// These are used only if an endpoint for either isn't specified
	defaultTraceEndpoint := fmt.Sprintf(DATADOG_TRACE_ENDPOINT, args.APISettings.Site)
	defaultMetricsEndpoint := fmt.Sprintf(DATADOG_METRICS_ENDPOINT, args.APISettings.Site)
	defaultLogsEndpoint := fmt.Sprintf(DATADOG_LOGS_ENDPOINT, args.APISettings.Site)

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}

	return &datadogOtelconfig.Config{
		ClientConfig:  *args.Client.Convert(),
		QueueSettings: q,
		BackOffConfig: *args.Retry.Convert(),
		TagsConfig: datadogOtelconfig.TagsConfig{
			Hostname: args.Hostname,
		},
		API:                      *args.APISettings.Convert(),
		Traces:                   *args.Traces.Convert(defaultTraceEndpoint),
		Metrics:                  *args.Metrics.Convert(defaultMetricsEndpoint),
		Logs:                     *args.Logs.Convert(defaultLogsEndpoint),
		HostMetadata:             *args.HostMetadata.Convert(),
		HostnameDetectionTimeout: args.HostnameDetectionTimeout,
		OnlyMetadata:             args.OnlyMetadata,
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.Queue.Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.APISettings.Key == "" {
		return errors.New("missing API key")
	}
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}
	datadogCfg := otelCfg.(*datadogOtelconfig.Config)
	return datadogCfg.Validate()
}
