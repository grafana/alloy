package controller

import (
	"fmt"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/featuregate"
)

// ComponentRegistry is a collection of registered components.
type ComponentRegistry interface {
	// Get looks up a component by name. It returns an error if the component does not exist or its usage is restricted,
	// for example, because of the component's stability level.
	Get(name string) (component.Registration, error)
}

type defaultComponentRegistry struct {
	minStability featuregate.Stability
}

// NewDefaultComponentRegistry creates a new [ComponentRegistry] which gets
// components registered to github.com/grafana/agent/component.
func NewDefaultComponentRegistry(minStability featuregate.Stability) ComponentRegistry {
	return defaultComponentRegistry{
		minStability: minStability,
	}
}

// Get retrieves a component using [component.Get]. It returns an error if the component does not exist,
// or if the component's stability is below the minimum required stability level.
func (reg defaultComponentRegistry) Get(name string) (component.Registration, error) {
	cr, exists := component.Get(name)
	if !exists {
		return component.Registration{}, fmt.Errorf("cannot find the definition of component name %q", name)
	}
	if err := featuregate.CheckAllowed(cr.Stability, reg.minStability, fmt.Sprintf("component %q", name)); err != nil {
		return component.Registration{}, err
	}
	return cr, nil
}

type registryMap struct {
	registrations map[string]component.Registration
	minStability  featuregate.Stability
}

// NewRegistryMap creates a new [ComponentRegistry] which uses a map to store components.
// Currently, it is only used in tests.
func NewRegistryMap(
	minStability featuregate.Stability,
	registrations map[string]component.Registration,
) ComponentRegistry {

	return &registryMap{
		registrations: registrations,
		minStability:  minStability,
	}
}

// Get retrieves a component using [component.Get].
func (m registryMap) Get(name string) (component.Registration, error) {
	reg, ok := m.registrations[name]
	if !ok {
		return component.Registration{}, fmt.Errorf("cannot find the definition of component name %q", name)
	}
	if err := featuregate.CheckAllowed(reg.Stability, m.minStability, fmt.Sprintf("component %q", name)); err != nil {
		return component.Registration{}, err
	}
	return reg, nil
}
