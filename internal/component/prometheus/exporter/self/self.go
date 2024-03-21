package self

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/agent"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.self",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "agent"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// Arguments holds values which are used to configured the prometheus.exporter.self component.
type Arguments struct{}

// Exports holds the values exported by the prometheus.exporter.self component.
type Exports struct{}

// SetToDefault implements river.Defaulter
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
}

func (a *Arguments) Convert() *agent.Config {
	return &agent.Config{}
}
