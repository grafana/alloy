// Package cloudflare provides an otelcol.receiver.cloudflare component.
package cloudflare

import (
	"errors"

	"github.com/alecthomas/units"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

const defaultMaxRequestBodySize = 20 * units.MiB

var (
	_ receiver.Arguments = Arguments{}
	_ syntax.Defaulter   = (*Arguments)(nil)
	_ syntax.Validator   = (*Arguments)(nil)
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.cloudflare",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := cloudflarereceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

type Arguments struct {
	Endpoint           string                      `alloy:"endpoint,attr"`
	Secret             string                      `alloy:"secret,attr,optional"`
	TimestampField     string                      `alloy:"timestamp_field,attr,optional"`
	TimestampFormat    string                      `alloy:"timestamp_format,attr,optional"`
	Separator          string                      `alloy:"separator,attr,optional"`
	Attributes         map[string]string           `alloy:"attributes,attr,optional"`
	MaxRequestBodySize units.Base2Bytes            `alloy:"max_request_body_size,attr,optional"`
	TLS                *otelcol.TLSServerArguments `alloy:"tls,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	cfg := cloudflarereceiver.NewFactory().CreateDefaultConfig().(*cloudflarereceiver.Config)
	*args = Arguments{
		TimestampField:     cfg.Logs.TimestampField,
		TimestampFormat:    cfg.Logs.TimestampFormat,
		Separator:          cfg.Logs.Separator,
		MaxRequestBodySize: defaultMaxRequestBodySize,
	}
}

func (args Arguments) receiverConfig() *cloudflarereceiver.Config {
	tlsCfg := args.TLS.Convert()
	cfg := cloudflarereceiver.NewFactory().CreateDefaultConfig().(*cloudflarereceiver.Config)
	cfg.Logs.Secret = args.Secret
	cfg.Logs.Endpoint = args.Endpoint
	cfg.Logs.TLS = tlsCfg.Get()
	cfg.Logs.Attributes = args.Attributes
	cfg.Logs.TimestampField = args.TimestampField
	cfg.Logs.TimestampFormat = args.TimestampFormat
	cfg.Logs.Separator = args.Separator
	cfg.Logs.MaxRequestBodySize = int64(args.MaxRequestBodySize)
	return cfg
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.MaxRequestBodySize <= 0 {
		return errors.New("max_request_body_size must be greater than 0")
	}
	otelCfg := args.receiverConfig()
	return otelCfg.Validate()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	otelCfg := args.receiverConfig()
	return otelCfg, nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	// Underlying receiver doesn't support debug metrics.
	// Return defaults (see: DebugMetricsArguments.SetToDefault)
	return otelcolCfg.DebugMetricsArguments{
		DisableHighCardinalityMetrics: true,
		Level:                         otelcolCfg.LevelDetailed,
	}
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}
