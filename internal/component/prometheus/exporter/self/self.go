package self

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/common"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/agent"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.self",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "self"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := common.HostNameInstanceKey() // if cannot resolve instance key, use the host name for self exporter
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// Arguments holds values which are used to configured the prometheus.exporter.self component.
type Arguments struct{}

// Exports holds the values exported by the prometheus.exporter.self component.
type Exports struct{}

// SetToDefault implements syntax.Defaulter
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
}

func (a *Arguments) Convert() *agent.Config {
	return &agent.Config{}
}
