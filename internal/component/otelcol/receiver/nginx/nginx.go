// Package nginx provides an otelcol.receiver.nginx component.
package nginx

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.nginx",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := nginxreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.nginx component.
type Arguments struct {
	Endpoint           string        `alloy:"endpoint,attr"`
	CollectionInterval time.Duration `alloy:"collection_interval,attr,optional"`
	InitialDelay       time.Duration `alloy:"initial_delay,attr,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	cfg := nginxreceiver.NewFactory().CreateDefaultConfig().(*nginxreceiver.Config)
	*args = Arguments{
		CollectionInterval: cfg.CollectionInterval,
		Endpoint:           cfg.Endpoint,
		InitialDelay:       cfg.InitialDelay,
	}
	args.DebugMetrics.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	cfg := nginxreceiver.NewFactory().CreateDefaultConfig().(*nginxreceiver.Config)
	cfg.ControllerConfig.CollectionInterval = args.CollectionInterval
	cfg.ControllerConfig.InitialDelay = args.InitialDelay
	cfg.ClientConfig.Endpoint = args.Endpoint
	return cfg, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
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
