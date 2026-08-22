//go:build !alloy_custom_components || alloy_component_discovery_ec2

package aws

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.ec2",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      EC2Arguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(EC2Arguments))
		},
	})
}
