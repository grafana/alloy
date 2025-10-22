// Package host_info provides an otelcol.connector.host_info component.
package host_info

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.host_info",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := NewFactory()
			return connector.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.host_info component.
type Arguments struct {
	HostIdentifiers      []string      `alloy:"host_identifiers,attr,optional"`
	MetricsFlushInterval time.Duration `alloy:"metrics_flush_interval,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ syntax.Validator    = (*Arguments)(nil)
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ connector.Arguments = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		HostIdentifiers:      []string{"host.id"},
		MetricsFlushInterval: 60 * time.Second,
	}
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if len(args.HostIdentifiers) == 0 {
		return fmt.Errorf("host_identifiers must not be empty")
	}

	if args.MetricsFlushInterval <= 0 {
		return fmt.Errorf("metrics_flush_interval must be greater than 0")
	}

	return nil
}

// Convert implements connector.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &Config{
		HostIdentifiers:      args.HostIdentifiers,
		MetricsFlushInterval: args.MetricsFlushInterval,
	}, nil
}

// Extensions implements connector.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements connector.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ConnectorType() int implements connector.Arguments.
func (Arguments) ConnectorType() int {
	return connector.ConnectorTracesToMetrics
}

// DebugMetricsConfig implements connector.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
