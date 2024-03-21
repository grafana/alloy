// Package host_info provides an otelcol.connector.host_info component.
package host_info

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/connector"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.host_info",
		Stability: featuregate.StabilityExperimental,
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
	HostIdentifiers      []string      `river:"host_identifiers,attr,optional"`
	MetricsFlushInterval time.Duration `river:"metrics_flush_interval,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var (
	_ river.Validator     = (*Arguments)(nil)
	_ river.Defaulter     = (*Arguments)(nil)
	_ connector.Arguments = (*Arguments)(nil)
)

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		HostIdentifiers:      []string{"host.id"},
		MetricsFlushInterval: 60 * time.Second,
	}
}

// Validate implements river.Validator.
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
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
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
