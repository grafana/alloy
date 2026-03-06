// Package awsfirehose provides an otelcol.receiver.awsfirehose component.
package awsfirehose

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.awsfirehose",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := awsfirehosereceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.awsfirehose component.
type Arguments struct {
	// Encoding identifies the encoding of records received from
	// Firehose. Defaults to telemetry-specific encodings: "cwlog"
	// for logs, and "cwmetrics" for metrics.
	Encoding string `alloy:"encoding,attr,optional"`

	// The access key to be checked on each request received.
	AccessKey alloytypes.Secret `alloy:"access_key,attr,optional"`

	HTTPServer otelcol.HTTPServerArguments `alloy:",squash"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// DefaultArguments holds default settings for otelcol.receiver.awsfirehose.
var DefaultArguments = Arguments{
	Encoding: "cwmetrics",
	HTTPServer: otelcol.HTTPServerArguments{
		Endpoint: "0.0.0.0:4433",
	},
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	httpServerConfig, err := args.HTTPServer.ConvertToPtr()
	if err != nil {
		return nil, err
	}
	return &awsfirehosereceiver.Config{
		Encoding:     args.Encoding,
		AccessKey:    configopaque.String(args.AccessKey),
		ServerConfig: *httpServerConfig,
	}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.HTTPServer.Extensions()
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
