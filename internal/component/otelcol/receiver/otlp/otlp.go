// Package otlp provides an otelcol.receiver.otlp component.
package otlp

import (
	"fmt"
	net_url "net/url"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.otlp",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := otlpreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.otlp component.
type Arguments struct {
	GRPC *GRPCServerArguments `alloy:"grpc,block,optional"`
	HTTP *HTTPConfigArguments `alloy:"http,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

type HTTPConfigArguments struct {
	HTTPServerArguments *otelcol.HTTPServerArguments `alloy:",squash"`

	// The URL path to receive traces on. If omitted "/v1/traces" will be used.
	TracesURLPath string `alloy:"traces_url_path,attr,optional"`

	// The URL path to receive metrics on. If omitted "/v1/metrics" will be used.
	MetricsURLPath string `alloy:"metrics_url_path,attr,optional"`

	// The URL path to receive logs on. If omitted "/v1/logs" will be used.
	LogsURLPath string `alloy:"logs_url_path,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *HTTPConfigArguments) Convert() *otlpreceiver.HTTPConfig {
	if args == nil {
		return nil
	}

	return &otlpreceiver.HTTPConfig{
		ServerConfig:   args.HTTPServerArguments.Convert(),
		TracesURLPath:  args.TracesURLPath,
		MetricsURLPath: args.MetricsURLPath,
		LogsURLPath:    args.LogsURLPath,
	}
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &otlpreceiver.Config{
		Protocols: otlpreceiver.Protocols{
			GRPC: (*otelcol.GRPCServerArguments)(args.GRPC).Convert(),
			HTTP: args.HTTP.Convert(),
		},
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

type (
	// GRPCServerArguments is used to configure otelcol.receiver.otlp with
	// component-specific defaults.
	GRPCServerArguments otelcol.GRPCServerArguments
)

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.HTTP != nil {
		if err := validateURL(args.HTTP.TracesURLPath, "traces_url_path"); err != nil {
			return err
		}
		if err := validateURL(args.HTTP.LogsURLPath, "logs_url_path"); err != nil {
			return err
		}
		if err := validateURL(args.HTTP.MetricsURLPath, "metrics_url_path"); err != nil {
			return err
		}
	}
	return nil
}

func validateURL(url string, urlName string) error {
	if url == "" {
		return fmt.Errorf("%s cannot be empty", urlName)
	}
	if _, err := net_url.Parse(url); err != nil {
		return fmt.Errorf("invalid %s: %w", urlName, err)
	}
	return nil
}

// SetToDefault implements syntax.Defaulter.
func (args *GRPCServerArguments) SetToDefault() {
	*args = GRPCServerArguments{
		Endpoint:  "localhost:4317",
		Transport: "tcp",

		ReadBufferSize: 512 * units.Kibibyte,
		// We almost write 0 bytes, so no need to tune WriteBufferSize.
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *HTTPConfigArguments) SetToDefault() {
	*args = HTTPConfigArguments{
		HTTPServerArguments: &otelcol.HTTPServerArguments{
			Endpoint:              "localhost:4318",
			CompressionAlgorithms: append([]string(nil), otelcol.DefaultCompressionAlgorithms...),
		},
		MetricsURLPath: "/v1/metrics",
		LogsURLPath:    "/v1/logs",
		TracesURLPath:  "/v1/traces",
	}
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
