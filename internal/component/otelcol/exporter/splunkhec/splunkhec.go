// package splunkhec provides an otel.exporter splunkhec component
// Maintainers for the Grafana Alloy wrapper:
// - @adlotsof
// - @PatMis16
package splunkhec

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.splunkhec",
		Community: true,
		Args:      config.SplunkHecArguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := splunkhecexporter.NewFactory()
			return exporter.New(opts, fact, args.(config.SplunkHecArguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

var _ exporter.Arguments = config.SplunkHecArguments{}
