//go:build !alloy_custom_components || alloy_component_discovery_lightsail

package aws

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.lightsail",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      LightsailArguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewLightsail(opts, args.(LightsailArguments))
		},
	})
}
