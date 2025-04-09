package controller

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

// ComponentRegistry is a collection of registered components.
type ComponentRegistry interface {
	// Get looks up a component by name. It returns an error if the component does not exist or its usage is restricted,
	// for example, because of the component's stability level.
	Get(name string) (component.Registration, error)
}

type defaultComponentRegistry struct {
	minStability featuregate.Stability
	community    bool
}

// NewDefaultComponentRegistry creates a new [ComponentRegistry] which gets
// components registered to github.com/grafana/alloy/internal/component.
func NewDefaultComponentRegistry(minStability featuregate.Stability, enableCommunityComps bool) ComponentRegistry {
	return defaultComponentRegistry{
		minStability: minStability,
		community:    enableCommunityComps,
	}
}

// Get retrieves a component using [component.Get]. It returns an error if the component does not exist,
// or if the component's stability is below the minimum required stability level.
func (reg defaultComponentRegistry) Get(name string) (component.Registration, error) {
	cr, exists := component.Get(name)
	if !exists {
		return component.Registration{}, fmt.Errorf("cannot find the definition of component name %q", name)
	}

	if cr.Community {
		if !reg.community {
			return component.Registration{}, fmt.Errorf("the component %q is a community component. Use the --feature.community-components.enabled command-line flag to enable community components", name)
		}
		return cr, nil // community components are not affected by feature stability
	}

	err := featuregate.CheckAllowed(cr.Stability, reg.minStability, fmt.Sprintf("component %q", name))
	if err != nil {
		return component.Registration{}, err
	}
	return cr, nil
}

type registryMap struct {
	registrations map[string]component.Registration
	minStability  featuregate.Stability
	community     bool
}

// NewRegistryMap creates a new [ComponentRegistry] which uses a map to store components.
// Currently, it is only used in tests.
func NewRegistryMap(
	minStability featuregate.Stability,
	community bool,
	registrations map[string]component.Registration,
) ComponentRegistry {

	return &registryMap{
		registrations: registrations,
		minStability:  minStability,
		community:     community,
	}
}

// Get retrieves a component using [component.Get].
func (m registryMap) Get(name string) (component.Registration, error) {
	reg, ok := m.registrations[name]
	if !ok {
		return component.Registration{}, fmt.Errorf("cannot find the definition of component name %q", name)
	}
	if reg.Community {
		if !m.community {
			return component.Registration{}, fmt.Errorf("the component %q is a community component. Use the --feature.community-components.enabled command-line flag to enable community components", name)
		}
		return reg, nil // community components are not affected by feature stability
	}

	err := featuregate.CheckAllowed(reg.Stability, m.minStability, fmt.Sprintf("component %q", name))
	if err != nil {
		return component.Registration{}, err
	}
	return reg, nil
}
