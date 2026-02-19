package runtime

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
)

// GetServiceConsumers implements [service.Host]. It returns a slice of
// [component.Component] and [service.Service]s which declared a dependency on
// the named service.
func (f *Runtime) GetServiceConsumers(serviceName string) []service.Consumer {
	consumers := serviceConsumersForGraph(f.loader.Graph(), serviceName, true)

	// Iterate through all modules to find other components that depend on the
	// service. Peer services aren't checked here, since the services are always
	// a subset of the services from the root controller.
	for _, mod := range f.modules.List() {
		moduleGraph := mod.f.loader.Graph()
		consumers = append(consumers, serviceConsumersForGraph(moduleGraph, serviceName, false)...)
	}

	return consumers
}

// GetService implements [service.Host]. It looks up a [service.Service] by
// name.
func (f *Runtime) GetService(name string) (service.Service, bool) {
	for _, svc := range f.opts.Services {
		if svc.Definition().Name == name {
			return svc, true
		}
	}
	return nil, false
}

func serviceConsumersForGraph(graph *dag.Graph, serviceName string, includePeerServices bool) []service.Consumer {
	serviceNode, _ := graph.GetByID(serviceName).(*controller.ServiceNode)
	if serviceNode == nil {
		return nil
	}
	dependants := graph.Dependants(serviceNode)

	consumers := make([]service.Consumer, 0, len(dependants))

	for _, consumer := range dependants {
		// Only return instances of component.Component and service.Service.
		switch consumer := consumer.(type) {
		case *controller.ServiceNode:
			if !includePeerServices {
				continue
			}

			if svc := consumer.Service(); svc != nil {
				consumers = append(consumers, service.Consumer{
					Type:  service.ConsumerTypeService,
					ID:    consumer.NodeID(),
					Value: svc,
				})
			}
		}
	}

	return consumers
}

// NewController returns a new, unstarted, isolated Alloy controller so that
// services can instantiate their own components.
func (f *Runtime) NewController(id string) (service.Controller, error) {
	c, err := newController(controllerOptions{
		Options: Options{
			ControllerID:         id,
			Logger:               f.opts.Logger,
			Tracer:               f.opts.Tracer,
			DataPath:             f.opts.DataPath,
			MinStability:         f.opts.MinStability,
			Reg:                  f.opts.Reg,
			Services:             f.opts.Services,
			EnableCommunityComps: f.opts.EnableCommunityComps,
			OnExportsChange:      nil, // NOTE(@tpaschalis, @wildum) The isolated controller shouldn't be able to export any values.
		},
		IsModule:       true,
		ModuleRegistry: newModuleRegistry(),
		WorkerPool:     f.opts.WorkerPool, // NOTE(@tpaschalis) Reuse the worker pool since the worker cleanup is triggered from the root controller.
	})
	if err != nil {
		return ServiceController{}, fmt.Errorf("failed to create new service controller: %w", err)
	}

	return ServiceController{f: c}, nil
}

type ServiceController struct {
	f *Runtime
}

func (sc ServiceController) Run(ctx context.Context) { sc.f.Run(ctx) }
func (sc ServiceController) LoadSource(b []byte, args map[string]any, configPath string) (*ast.File, error) {
	source, err := ParseSource("", b)
	if err != nil {
		return nil, err
	}
	return source.SourceFiles()[""], sc.f.LoadSource(source, args, configPath)
}
func (sc ServiceController) Ready() bool { return sc.f.Ready() }

func (sc ServiceController) GetHost() service.Host { return sc.f }
