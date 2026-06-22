// Package elasticsearch provides an otelcol.exporter.elasticsearch component.
package elasticsearch

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/elasticsearch/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.elasticsearch",
		Community: true,
		Args:      config.ElasticsearchArguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := elasticsearchexporter.NewFactory()
			return exporter.New(opts, fact, args.(config.ElasticsearchArguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

var _ exporter.Arguments = config.ElasticsearchArguments{}
