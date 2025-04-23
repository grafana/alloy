package splunkhec

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelconfig "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.splunkhec",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			f := splunkhecreceiver.NewFactory()
			return receiver.New(opts, f, args.(Arguments))
		},
	})
}

type Arguments struct{}

// Convert implements receiver.Arguments.
func (a Arguments) Convert() (otelcomponent.Config, error) {
	panic("unimplemented")
}

// DebugMetricsConfig implements receiver.Arguments.
func (a Arguments) DebugMetricsConfig() otelconfig.DebugMetricsArguments {
	panic("unimplemented")
}

// Exporters implements receiver.Arguments.
func (a Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	panic("unimplemented")
}

// Extensions implements receiver.Arguments.
func (a Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	panic("unimplemented")
}

// NextConsumers implements receiver.Arguments.
func (a Arguments) NextConsumers() *otelcol.ConsumerArguments {
	panic("unimplemented")
}
