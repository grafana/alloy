// Package azure provides an otelcol.encoding.azure component.
package azure

import (
	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.encoding.azure",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   extension.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := azureencodingextension.NewFactory()
			return extension.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.encoding.azure component.
type Arguments struct {
	Logs    LogsArguments    `alloy:"logs,block,optional"`
	Metrics MetricsArguments `alloy:"metrics,block,optional"`
	Traces  TracesArguments  `alloy:"traces,block,optional"`
}

// LogsArguments configures decoding of Azure resource logs.
type LogsArguments struct {
	TimeFormats       []string `alloy:"time_formats,attr,optional"`
	IncludeCategories []string `alloy:"include_categories,attr,optional"`
	ExcludeCategories []string `alloy:"exclude_categories,attr,optional"`
}

// MetricsArguments configures decoding of Azure metrics.
type MetricsArguments struct {
	TimeFormats  []string `alloy:"time_formats,attr,optional"`
	Aggregations []string `alloy:"aggregations,attr,optional"`
}

// TracesArguments configures decoding of Azure traces.
type TracesArguments struct {
	TimeFormats []string `alloy:"time_formats,attr,optional"`
}

var (
	_ extension.Arguments = Arguments{}
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ syntax.Validator    = (*Arguments)(nil)
)

// ArgumentsFromConfig constructs component arguments from an Azure encoding extension config.
func ArgumentsFromConfig(cfg *azureencodingextension.Config) Arguments {
	return Arguments{
		Logs: LogsArguments{
			TimeFormats:       cfg.Logs.TimeFormats,
			IncludeCategories: cfg.Logs.IncludeCategories,
			ExcludeCategories: cfg.Logs.ExcludeCategories,
		},
		Metrics: MetricsArguments{
			TimeFormats:  cfg.Metrics.TimeFormats,
			Aggregations: toStrings(cfg.Metrics.Aggregations),
		},
		Traces: TracesArguments{
			TimeFormats: cfg.Traces.TimeFormats,
		},
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	defaultCfg := azureencodingextension.NewFactory().CreateDefaultConfig().(*azureencodingextension.Config)
	*args = ArgumentsFromConfig(defaultCfg)
}

func (args Arguments) otelConfig() *azureencodingextension.Config {
	cfg := azureencodingextension.NewFactory().CreateDefaultConfig().(*azureencodingextension.Config)
	cfg.Logs.TimeFormats = args.Logs.TimeFormats
	cfg.Logs.IncludeCategories = args.Logs.IncludeCategories
	cfg.Logs.ExcludeCategories = args.Logs.ExcludeCategories
	cfg.Metrics.TimeFormats = args.Metrics.TimeFormats
	cfg.Metrics.Aggregations = fromStrings(cfg.Metrics.Aggregations, args.Metrics.Aggregations)
	cfg.Traces.TimeFormats = args.Traces.TimeFormats
	return cfg
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// The upstream config only defines Validate on nested structs.
	return confmap.Validate(args.otelConfig())
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
	cfg := otelcolCfg.DebugMetricsArguments{}
	cfg.SetToDefault()
	return cfg
}

// toStrings converts a slice of string-based values to a string slice.
func toStrings[T ~string](in []T) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, string(v))
	}
	return out
}

// fromStrings converts a string slice to a slice of string-based values.
// The first argument is only used to infer the element type,
// as upstream declares it in an internal package.
func fromStrings[T ~string](_ []T, in []string) []T {
	if in == nil {
		return nil
	}
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, T(v))
	}
	return out
}
