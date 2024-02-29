package flow

import (
	"context"

	"github.com/grafana/agent/internal/flow/internal/controller"
	"github.com/grafana/agent/internal/flow/internal/dag"
	"github.com/grafana/agent/internal/flow/internal/worker"
	"github.com/grafana/agent/internal/service"
)

// GetServiceConsumers implements [service.Host]. It returns a slice of
// [component.Component] and [service.Service]s which declared a dependency on
// the named service.
func (f *Flow) GetServiceConsumers(serviceName string) []service.Consumer {
	consumers := serviceConsumersForGraph(f.loader.OriginalGraph(), serviceName, true)

	// Iterate through all modules to find other components that depend on the
	// service. Peer services aren't checked here, since the services are always
	// a subset of the services from the root controller.
	for _, mod := range f.modules.List() {
		moduleGraph := mod.f.loader.OriginalGraph()
		consumers = append(consumers, serviceConsumersForGraph(moduleGraph, serviceName, false)...)
	}

	return consumers
}

// GetService implements [service.Host]. It looks up a [service.Service] by
// name.
func (f *Flow) GetService(name string) (service.Service, bool) {
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

// NewController returns a new, unstarted, isolated Flow controller so that
// services can instantiate their own components.
func (f *Flow) NewController(id string) service.Controller {
	return serviceController{
		f: newController(controllerOptions{
			Options: Options{
				ControllerID:    id,
				Logger:          f.opts.Logger,
				Tracer:          f.opts.Tracer,
				DataPath:        f.opts.DataPath,
				MinStability:    f.opts.MinStability,
				Reg:             f.opts.Reg,
				Services:        f.opts.Services,
				OnExportsChange: nil, // NOTE(@tpaschalis, @wildum) The isolated controller shouldn't be able to export any values.
			},
			IsModule:       true,
			ModuleRegistry: newModuleRegistry(),
			WorkerPool:     worker.NewDefaultWorkerPool(),
		}),
	}
}

type serviceController struct {
	f *Flow
}

func (sc serviceController) Run(ctx context.Context) { sc.f.Run(ctx) }
func (sc serviceController) LoadSource(b []byte, args map[string]any) error {
	source, err := ParseSource("", b)
	if err != nil {
		return err
	}
	return sc.f.LoadSource(source, args)
}
func (sc serviceController) Ready() bool { return sc.f.Ready() }
