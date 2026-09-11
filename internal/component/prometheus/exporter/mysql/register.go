//go:build !alloy_custom_components || alloy_component_prometheus_exporter_mysql

package mysql

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mysql",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "mysql"),
	})
}
