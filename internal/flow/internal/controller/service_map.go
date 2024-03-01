package controller

import (
	"github.com/grafana/agent/internal/service"
	"golang.org/x/exp/maps"
)

// ServiceMap is a map of service name to services.
type ServiceMap map[string]service.Service

// NewServiceMap creates a ServiceMap from a slice of services.
func NewServiceMap(services []service.Service) ServiceMap {
	m := make(ServiceMap, len(services))
	for _, svc := range services {
		name := svc.Definition().Name
		m[name] = svc
	}
	return m
}

// Get looks up a service by name.
func (sm ServiceMap) Get(name string) (svc service.Service, found bool) {
	svc, found = sm[name]
	return svc, found
}

// List returns a slice of all the services.
func (sm ServiceMap) List() []service.Service { return maps.Values(sm) }

// FilterByName creates a new ServiceMap where services that are not defined in
// keepNames are removed.
func (sm ServiceMap) FilterByName(keepNames []string) ServiceMap {
	newMap := make(ServiceMap, len(keepNames))

	for _, name := range keepNames {
		if svc, found := sm[name]; found {
			newMap[name] = svc
		}
	}

	return newMap
}
