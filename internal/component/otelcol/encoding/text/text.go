// Package text provides an otelcol.encoding.text component.
package text

import (
	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name: "otelcol.encoding.text",
		// TODO: Promote to public preview after the new Alloy component matures; the upstream extension is beta.
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   extension.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := textencodingextension.NewFactory()
			return extension.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.encoding.text component.
type Arguments struct {
	Encoding              string `alloy:"encoding,attr,optional"`
	MarshalingSeparator   string `alloy:"marshaling_separator,attr,optional"`
	UnmarshalingSeparator string `alloy:"unmarshaling_separator,attr,optional"`
}

var (
	_ extension.Arguments = Arguments{}
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ syntax.Validator    = (*Arguments)(nil)
)

// ArgumentsFromConfig constructs component arguments from a text encoding extension config.
func ArgumentsFromConfig(cfg *textencodingextension.Config) Arguments {
	return Arguments{
		Encoding:              cfg.Encoding,
		MarshalingSeparator:   cfg.MarshalingSeparator,
		UnmarshalingSeparator: cfg.UnmarshalingSeparator,
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	defaultCfg := textencodingextension.NewFactory().CreateDefaultConfig().(*textencodingextension.Config)
	*args = ArgumentsFromConfig(defaultCfg)
}

func (args Arguments) otelConfig() *textencodingextension.Config {
	cfg := textencodingextension.NewFactory().CreateDefaultConfig().(*textencodingextension.Config)
	cfg.Encoding = args.Encoding
	cfg.MarshalingSeparator = args.MarshalingSeparator
	cfg.UnmarshalingSeparator = args.UnmarshalingSeparator
	return cfg
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	return args.otelConfig().Validate()
}

// Convert implements extension.Arguments.
func (args Arguments) Convert(_ component.Options) (otelcomponent.Config, error) {
	return args.otelConfig(), nil
}

// ExportsHandler implements extension.Arguments.
func (args Arguments) ExportsHandler() bool {
	return true
}

// Extensions implements extension.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements extension.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements extension.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	// The underlying extension doesn't support debug metrics.
	// Return defaults (see: DebugMetricsArguments.SetToDefault).
	return otelcolCfg.DebugMetricsArguments{
		DisableHighCardinalityMetrics: true,
		Level:                         otelcolCfg.LevelDetailed,
	}
}
