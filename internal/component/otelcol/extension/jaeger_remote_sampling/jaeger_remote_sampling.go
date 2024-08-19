package jaeger_remote_sampling

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/extension/jaeger_remote_sampling/internal/jaegerremotesampling"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.extension.jaeger_remote_sampling",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := jaegerremotesampling.NewFactory()

			return extension.New(opts, fact, args.(Arguments))
		},
	})
}

type (
	// GRPCServerArguments is used to configure otelcol.extension.jaeger_remote_sampling with
	// component-specific defaults.
	GRPCServerArguments otelcol.GRPCServerArguments

	// HTTPServerArguments is used to configure otelcol.extension.jaeger_remote_sampling with
	// component-specific defaults.
	HTTPServerArguments otelcol.HTTPServerArguments
)

// Arguments configures the otelcol.extension.jaegerremotesampling component.
type Arguments struct {
	GRPC *GRPCServerArguments `alloy:"grpc,block,optional"`
	HTTP *HTTPServerArguments `alloy:"http,block,optional"`

	Source ArgumentsSource `alloy:"source,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type ArgumentsSource struct {
	Content        string               `alloy:"content,attr,optional"`
	Remote         *GRPCClientArguments `alloy:"remote,block,optional"`
	File           string               `alloy:"file,attr,optional"`
	ReloadInterval time.Duration        `alloy:"reload_interval,attr,optional"`
}

var (
	_ extension.Arguments = Arguments{}
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

// Convert implements extension.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &jaegerremotesampling.Config{
		HTTPServerConfig: (*otelcol.HTTPServerArguments)(args.HTTP).Convert(),
		GRPCServerConfig: (*otelcol.GRPCServerArguments)(args.GRPC).Convert(),
		Source: jaegerremotesampling.Source{
			Remote:         (*otelcol.GRPCClientArguments)(args.Source.Remote).Convert(),
			File:           args.Source.File,
			ReloadInterval: args.Source.ReloadInterval,
			Contents:       args.Source.Content,
		},
	}, nil
}

// Extensions implements extension.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements extension.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements extension.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.GRPC == nil && a.HTTP == nil {
		return fmt.Errorf("http or grpc must be configured to serve the sampling document")
	}

	return nil
}

// Validate implements syntax.Validator.
func (a *ArgumentsSource) Validate() error {
	// remote config, local file and contents are all mutually exclusive
	sourcesSet := 0
	if a.Content != "" {
		sourcesSet++
	}
	if a.File != "" {
		sourcesSet++
	}
	if a.Remote != nil {
		sourcesSet++
	}

	if sourcesSet == 0 {
		return fmt.Errorf("one of contents, file or remote must be configured")
	}
	if sourcesSet > 1 {
		return fmt.Errorf("only one of contents, file or remote can be configured")
	}

	return nil
}

// SetToDefault implements syntax.Defaulter.
func (args *GRPCServerArguments) SetToDefault() {
	*args = GRPCServerArguments{
		Endpoint:  "0.0.0.0:14250",
		Transport: "tcp",
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *HTTPServerArguments) SetToDefault() {
	*args = HTTPServerArguments{
		Endpoint:              "0.0.0.0:5778",
		CompressionAlgorithms: append([]string(nil), otelcol.DefaultCompressionAlgorithms...),
	}
}

// GRPCClientArguments is used to configure
// otelcol.extension.jaeger_remote_sampling with
// component-specific defaults.
type GRPCClientArguments otelcol.GRPCClientArguments

// SetToDefault implements syntax.Defaulter.
func (args *GRPCClientArguments) SetToDefault() {
	*args = GRPCClientArguments{
		Headers:         map[string]string{},
		Compression:     otelcol.CompressionTypeGzip,
		WriteBufferSize: 512 * 1024,
		BalancerName:    otelcol.DefaultBalancerName,
	}
}
