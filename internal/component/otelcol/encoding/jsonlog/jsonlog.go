// Package jsonlog provides an otelcol.encoding.jsonlog component.
package jsonlog

import (
	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.encoding.jsonlog",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   extension.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := jsonlogencodingextension.NewFactory()
			return extension.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.encoding.jsonlog component.
type Arguments struct {
	ArrayMode bool   `alloy:"array_mode,attr,optional"`
	Mode      string `alloy:"mode,attr,optional"`
}

var (
	_ extension.Arguments = Arguments{}
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ syntax.Validator    = (*Arguments)(nil)
)

// ArgumentsFromConfig constructs component arguments from a JSON log encoding extension config.
func ArgumentsFromConfig(cfg *jsonlogencodingextension.Config) Arguments {
	return Arguments{
		ArrayMode: cfg.ArrayMode,
		Mode:      string(cfg.Mode),
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	defaultCfg := jsonlogencodingextension.NewFactory().CreateDefaultConfig().(*jsonlogencodingextension.Config)
	*args = ArgumentsFromConfig(defaultCfg)
}

func (args Arguments) otelConfig() *jsonlogencodingextension.Config {
	cfg := jsonlogencodingextension.NewFactory().CreateDefaultConfig().(*jsonlogencodingextension.Config)
	cfg.ArrayMode = args.ArrayMode
	cfg.Mode = jsonlogencodingextension.JSONEncodingMode(args.Mode)
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
