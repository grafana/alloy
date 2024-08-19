// Package datadog provides an otelcol.receiver.datadog component.
package datadog

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.datadog",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := datadogreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.datadog component.
type Arguments struct {
	HTTPServer otelcol.HTTPServerArguments `alloy:",squash"`

	ReadTimeout time.Duration `alloy:"read_timeout,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		HTTPServer: otelcol.HTTPServerArguments{
			Endpoint:              "localhost:8126",
			CompressionAlgorithms: append([]string(nil), otelcol.DefaultCompressionAlgorithms...),
		},
		ReadTimeout: 60 * time.Second,
	}
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &datadogreceiver.Config{
		ServerConfig: *args.HTTPServer.Convert(),
		ReadTimeout:  args.ReadTimeout,
	}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
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
