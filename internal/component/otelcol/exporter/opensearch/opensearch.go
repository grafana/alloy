// Package opensearch provides an otelcol.exporter.opensearch component.
package opensearch

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/opensearch/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.opensearch",
		Community: true,
		Args:      config.OpenSearchArguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := opensearchexporter.NewFactory()
			return exporter.New(opts, fact, args.(config.OpenSearchArguments), exporter.TypeSignalConstFunc(exporter.TypeLogs|exporter.TypeTraces))
		},
	})
}

var _ exporter.Arguments = config.OpenSearchArguments{}
