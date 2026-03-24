// Package datadog provides an otelcol.receiver.datadog component.
package datadog

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/pipeline"
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

	ReadTimeout      time.Duration `alloy:"read_timeout,attr,optional"`
	TraceIDCacheSize int           `alloy:"trace_id_cache_size,attr,optional"`

	Intake *IntakeArguments `alloy:"intake,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// IntakeArguments controls the /intake endpoint behavior.
type IntakeArguments struct {
	// Behavior is required; allowed values are "disable" or "proxy".
	Behavior string          `alloy:"behavior,attr"`
	Proxy    *ProxyArguments `alloy:"proxy,block,optional"`
}

// ProxyArguments controls how the /intake proxy operates.
type ProxyArguments struct {
	API APIArguments `alloy:"api,block"`
}

// APIArguments configures the Datadog API connection for the intake proxy.
type APIArguments struct {
	Key              alloytypes.Secret `alloy:"key,attr"`
	Site             string            `alloy:"site,attr,optional"`
	FailOnInvalidKey bool              `alloy:"fail_on_invalid_key,attr,optional"`
}

var _ receiver.Arguments = Arguments{}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Intake != nil {
		if err := args.Intake.Validate(); err != nil {
			return err
		}
	}

	// Validate the converted upstream config. The upstream Validate() is not
	// called automatically by the Alloy receiver framework, so we call it
	// explicitly here. This also avoids duplicating upstream validation logic
	// (e.g. allowed intake behavior values) which may evolve over time.
	cfg, err := args.Convert()
	if err != nil {
		return err
	}
	return cfg.(*datadogreceiver.Config).Validate()
}

// Validate checks IntakeArguments constraints documented in the component reference.
func (args *IntakeArguments) Validate() error {
	if args.Behavior == "proxy" && args.Proxy == nil {
		return fmt.Errorf("a proxy block with an api block is required when intake behavior is %q", args.Behavior)
	}
	return nil
}

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
	convertedHttpServer, err := args.HTTPServer.ConvertToPtr()
	if err != nil {
		return nil, err
	}

	cfg := &datadogreceiver.Config{
		ServerConfig:     *convertedHttpServer,
		ReadTimeout:      args.ReadTimeout,
		TraceIDCacheSize: args.TraceIDCacheSize,
	}

	if args.Intake != nil {
		cfg.Intake = args.Intake.Convert()
	}

	return cfg, nil
}

func (args *IntakeArguments) Convert() datadogreceiver.IntakeConfig {
	ic := datadogreceiver.IntakeConfig{
		Behavior: args.Behavior,
	}
	if args.Behavior == "proxy" && args.Proxy != nil {
		apiSite := args.Proxy.API.Site
		if apiSite == "" {
			apiSite = datadogconfig.DefaultSite
		}

		ic.Proxy = datadogreceiver.ProxyConfig{
			API: datadogconfig.APIConfig{
				Key:              configopaque.String(args.Proxy.API.Key),
				Site:             apiSite,
				FailOnInvalidKey: args.Proxy.API.FailOnInvalidKey,
			},
		}
	}
	return ic
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
