// Package cloudflare provides an otelcol.receiver.cloudflare component.
package cloudflare

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.cloudflare",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := cloudflarereceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}
