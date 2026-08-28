package fluentforward

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolConfig "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"
	collectorComponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.fluentforward",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := fluentforwardreceiver.NewFactory()
			return receiver.New(opts, factory, args.(Arguments))
		},
	})
}

var (
	_ receiver.Arguments = (*Arguments)(nil)
	_ syntax.Defaulter   = (*Arguments)(nil)
	_ syntax.Validator   = (*Arguments)(nil)
)

type Arguments struct {
	// The address to listen on for incoming Fluent Forward events.  Should be
	// of the form `<ip addr>:<port>` (TCP) or `unix://<socket_path>` (Unix
	// domain socket).
	Endpoint string `alloy:"endpoint,attr"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolConfig.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *ConsumerArguments `alloy:"output,block"`
}

func (a *Arguments) Validate() error {
	if a.Endpoint == "" {
		return fmt.Errorf("endpoint must not be empty")
	}
	return nil
}

func (a *Arguments) SetToDefault() {
	a.DebugMetrics.SetToDefault()
}

type ConsumerArguments struct {
	Logs []otelcol.Consumer `alloy:"logs,attr,optional"`
}

func (a Arguments) Convert() (collectorComponent.Config, error) {
	cfg := &fluentforwardreceiver.Config{
		ListenAddress: a.Endpoint,
	}
	return cfg, nil
}

func (a Arguments) DebugMetricsConfig() otelcolConfig.DebugMetricsArguments {
	return a.DebugMetrics
}

func (a Arguments) Exporters() map[pipeline.Signal]map[collectorComponent.ID]collectorComponent.Component {
	return nil
}

func (a Arguments) Extensions() map[collectorComponent.ID]collectorComponent.Component {
	return nil
}

func (a Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return &otelcol.ConsumerArguments{
		Logs: a.Output.Logs,
	}
}
