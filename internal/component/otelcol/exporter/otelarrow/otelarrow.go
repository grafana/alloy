// Package otelarrow provides an otelcol.exporter.otelarrow component.
package otelarrow

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	otelpexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

// Arguments configures the otelcol.exporter.otelarrow component.
type Arguments struct {
	Timeout                  time.Duration                    `alloy:"timeout,attr,optional"`
	Queue                    otelcol.QueueArguments           `alloy:"sending_queue,block,optional"`
	Retry                    otelcol.RetryArguments           `alloy:"retry_on_failure,block,optional"`
	Arrow                    ArrowArguments                   `alloy:"arrow,block"`
	MetadataKeys             []string                         `alloy:"metadata_keys,attr,optional"`
	MetadataCardinalityLimit uint32                           `alloy:"metadata_cardinality_limit,attr,optional"`
	DebugMetrics             otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
	Client                   GRPCClientArguments              `alloy:"client,block"`
}

// SetToDefault populates Arguments with default values.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Timeout:                  otelcol.DefaultTimeout,
		Arrow:                    defaultArrowArguments(),
		MetadataCardinalityLimit: 1000, // Default from otelarrowexporter README
	}
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert transforms Arguments into a config for otelarrowexporter.
// This method maps the Arguments fields to the corresponding fields in otelarrowexporter.Config,
// ensuring type compatibility and preventing interface conversion panics.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	// Convert GRPC client settings
	clientArgs := *(*otelcol.GRPCClientArguments)(&args.Client)
	convertedClientArgs, err := clientArgs.Convert()
	if err != nil {
		return nil, err
	}

	// Create the config from the factory to ensure default values
	factory := otelarrowexporter.NewFactory()
	defaultCfg := factory.CreateDefaultConfig().(*otelarrowexporter.Config)

	// Apply our specific settings
	cfg := defaultCfg

	// Set timeout using the correct field name
	cfg.TimeoutSettings = otelpexporterhelper.TimeoutConfig{
		Timeout: args.Timeout,
	}

	cfg.QueueSettings = *args.Queue.Convert()
	cfg.RetryConfig = *args.Retry.Convert()
	cfg.ClientConfig = *convertedClientArgs
	cfg.MetadataKeys = args.MetadataKeys
	cfg.MetadataCardinalityLimit = args.MetadataCardinalityLimit

	// Update specific arrow settings without directly accessing internal types
	cfg.Arrow.NumStreams = args.Arrow.NumStreams
	cfg.Arrow.MaxStreamLifetime = args.Arrow.MaxStreamLifetime
	cfg.Arrow.PayloadCompression = args.Arrow.PayloadCompression
	cfg.Arrow.Disabled = args.Arrow.Disabled
	cfg.Arrow.DisableDowngrade = args.Arrow.DisableDowngrade

	// Note: we rely on the default config from the factory for settings
	// that require internal types (Prioritizer, Zstd detailed configuration)

	return cfg, nil
}

// Extensions returns any gRPC client extensions (e.g., authentication).
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return (*otelcol.GRPCClientArguments)(&args.Client).Extensions()
}

// Exporters returns nil as this component does not use additional exporters.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig provides the debug metrics settings.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// ArrowArguments defines Arrow-specific configuration options.
type ArrowArguments struct {
	Disabled           bool                   `alloy:"disabled,attr,optional"`
	DisableDowngrade   bool                   `alloy:"disable_downgrade,attr,optional"`
	MaxStreamLifetime  time.Duration          `alloy:"max_stream_lifetime,attr,optional"`
	NumStreams         int                    `alloy:"num_streams,attr,optional"`
	Prioritizer        string                 `alloy:"prioritizer,attr,optional"`
	Zstd               ZstdEncoderConfig      `alloy:"zstd,block,optional"`
	PayloadCompression configcompression.Type `alloy:"payload_compression,attr,optional"`
}

// SetToDefault sets default values for ArrowArguments.
func (a *ArrowArguments) SetToDefault() {
	*a = ArrowArguments{
		Disabled:           false,
		DisableDowngrade:   false,
		MaxStreamLifetime:  30 * time.Second,
		NumStreams:         1, // Simplified default; could use max(1, NumCPU()/2)
		Prioritizer:        "leastloaded",
		PayloadCompression: configcompression.Type("none"),
		Zstd:               DefaultZstdEncoderConfig(),
	}
}

// defaultArrowArguments provides a helper to create default ArrowArguments.
func defaultArrowArguments() ArrowArguments {
	var aa ArrowArguments
	aa.SetToDefault()
	return aa
}

// ZstdEncoderConfig configures Zstd compression settings.
type ZstdEncoderConfig struct {
	Level         int    `alloy:"level,attr,optional"`           // 1-10, default 5
	WindowSizeMib uint32 `alloy:"window_size_mib,attr,optional"` // Default 0
	Concurrency   uint   `alloy:"concurrency,attr,optional"`     // Default 1
}

// DefaultZstdEncoderConfig returns default Zstd settings.
func DefaultZstdEncoderConfig() ZstdEncoderConfig {
	return ZstdEncoderConfig{
		Level:         5,
		WindowSizeMib: 0,
		Concurrency:   1,
	}
}

// GRPCClientArguments aliases otelcol.GRPCClientArguments for component-specific defaults.
type GRPCClientArguments otelcol.GRPCClientArguments

// SetToDefault sets default gRPC client settings.
func (args *GRPCClientArguments) SetToDefault() {
	*args = GRPCClientArguments{
		Headers:         map[string]string{},
		Compression:     otelcol.CompressionTypeGzip,
		WriteBufferSize: 512 * 1024,
		BalancerName:    otelcol.DefaultBalancerName,
	}
}

// Package-level initialization to register the component.
func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.otelarrow",
		Stability: featuregate.StabilityExperimental, // Matches README stability: beta
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := otelarrowexporter.NewFactory()
			// Directly return the exporter, as it implements component.Component.
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}
