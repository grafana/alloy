package cloudflare

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcol "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
)

var _ receiver.Arguments = Arguments{}

type Arguments struct {
	// TODO
}

// Convert implements receiver.Arguments.
func (a Arguments) Convert() (component.Config, error) {
	panic("unimplemented")
}

// DebugMetricsConfig implements receiver.Arguments.
func (a Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	panic("unimplemented")
}

// Exporters implements receiver.Arguments.
func (a Arguments) Exporters() map[pipeline.Signal]map[component.ID]component.Component {
	panic("unimplemented")
}

// Extensions implements receiver.Arguments.
func (a Arguments) Extensions() map[component.ID]component.Component {
	panic("unimplemented")
}

// NextConsumers implements receiver.Arguments.
func (a Arguments) NextConsumers() *otelcol.ConsumerArguments {
	panic("unimplemented")
}
