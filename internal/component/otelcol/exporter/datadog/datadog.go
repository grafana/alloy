// Package otlphttp provides an otelcol.exporter.otlphttp component.

package datadog

import (
	"errors"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	otelpexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.datadog",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := datadogexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.datadog component.
type Arguments struct {
	Timeout time.Duration          `alloy:"timeout,attr,optional"`
	Queue   otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry   otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// v0.96 of dd exporter doesn't support full http client yet
	SkipTLS      bool   `alloy:"insecure_skip_verify,attr,optional"`
	OnlyMetadata bool   `alloy:"only_metadata,attr,optional"`
	Hostname     string `alloy:"hostname,attr,optional"`

	// Datadog specific configuration settings
	APISettings DatadogAPISettings `alloy:"api,block"`
}

type TracesConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog intake server to send traces to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `alloy:",squash"`

	IgnoreResources []string `alloy:"ignore_resources"`

	SpanNameRemappings map[string]string `alloy:"span_name_remappings"`

	SpanNameAsResourceName bool `alloy:"span_name_as_resource_name"`

	ComputeStatsBySpanKind bool `mapstructure:"compute_stats_by_span_kind"`

	PeerServiceAggregation bool `mapstructure:"peer_service_aggregation"`

	PeerTagsAggregation bool `mapstructure:"peer_tags_aggregation"`

	PeerTags []string `mapstructure:"peer_tags"`

	TraceBuffer int `mapstructure:"trace_buffer"`

	flushInterval float64
}

// DatadogAPISettings holds the configuration settings for the Datadog API.
type DatadogAPISettings struct {
	Key              configopaque.String `alloy:"api_key,attr"`
	Site             string              `alloy:"site,attr,optional"` // Default value of exporter is "datadoghq.com"
	FailOnInvalidKey bool                `alloy:"fail_on_invalid_key,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

const (
	EncodingProto string = "proto"
	EncodingJSON  string = "json"
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Encoding: EncodingProto,
	}
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &datadogexporter.Config{
		TimeoutSettings: otelpexporterhelper.TimeoutSettings{
			Timeout: args.Timeout,
		},
		QueueSettings: *args.Queue.Convert(),
		BackOffConfig: *args.Retry.Convert(),
		API: datadogexporter.APIConfig{
			Key:              args.Key,
			Site:             args.Site,
			FailOnInvalidKey: args.FailOnInvalidKey,
		},
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return (*otelcol.HTTPClientArguments)(&args.Client).Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Client.Endpoint == "" && args.TracesEndpoint == "" && args.MetricsEndpoint == "" && args.LogsEndpoint == "" {
		return errors.New("at least one endpoint must be specified")
	}
	if args.Encoding != EncodingProto && args.Encoding != EncodingJSON {
		return fmt.Errorf("invalid encoding type %s", args.Encoding)
	}
	return nil
}

// HTTPClientArguments is used to configure otelcol.exporter.otlphttp with
// component-specific defaults.
type HTTPClientArguments otelcol.HTTPClientArguments

// Default server settings.
var (
	DefaultMaxIdleConns    = 100
	DefaultIdleConnTimeout = 90 * time.Second
)

// SetToDefault implements syntax.Defaulter.
func (args *HTTPClientArguments) SetToDefault() {
	maxIdleConns := DefaultMaxIdleConns
	idleConnTimeout := DefaultIdleConnTimeout
	*args = HTTPClientArguments{
		MaxIdleConns:    &maxIdleConns,
		IdleConnTimeout: &idleConnTimeout,

		Timeout:          30 * time.Second,
		Headers:          map[string]string{},
		Compression:      otelcol.CompressionTypeGzip,
		ReadBufferSize:   0,
		WriteBufferSize:  512 * 1024,
		HTTP2PingTimeout: 15 * time.Second,
	}
}
