// Package cloudflare provides an otelcol.receiver.cloudflare component.
package cloudflare

import (
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

type LogsConfig struct {
	Secret          string                      `alloy:"secret,attr"`
	Endpoint        string                      `alloy:"endpoint,attr"`
	TLS             *otelcol.TLSServerArguments `alloy:"tls,block,optional"`
	Attributes      map[string]string           `alloy:"attributes,attr,optional"`
	TimestampField  string                      `alloy:"timestamp_field,attr,optional"`
	TimestampFormat string                      `alloy:"timestamp_format,attr,optional"`
	Separator       string                      `alloy:"separator,attr,optional"`
}

func (lc LogsConfig) Convert() cloudflarereceiver.LogsConfig {
	tlsCfg := lc.TLS.Convert()
	return cloudflarereceiver.LogsConfig{
		Secret:          lc.Secret,
		Endpoint:        lc.Endpoint,
		TLS:             tlsCfg.Get(),
		Attributes:      lc.Attributes,
		TimestampField:  lc.TimestampField,
		TimestampFormat: lc.TimestampFormat,
		Separator:       lc.Separator,
	}
}

func (lc *LogsConfig) SetToDefault() {
	// Although otel's receiver already initializes defaults of downstream config,
	// let's do it as well to avoid breaking changes if defauls are changed in upstream.
	if lc.TimestampField == "" {
		lc.TimestampField = "EdgeStartTimestamp"
	}

	if lc.TimestampFormat == "" {
		lc.TimestampFormat = "rfc3339"
	}

	if lc.Separator == "" {
		lc.Separator = "."
	}
}

type Arguments struct {
	Logs LogsConfig `alloy:"logs,block"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Logs.SetToDefault()
}

func (args Arguments) receiverConfig() *cloudflarereceiver.Config {
	return &cloudflarereceiver.Config{
		Logs: args.Logs.Convert(),
	}
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
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
	return nil
}
