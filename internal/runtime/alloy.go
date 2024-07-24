// Package runtime implements the Alloy component graph system. Alloy configuration
// sources are parsed from Alloy syntax, which contain a listing of components to run.
//
// # Components
//
// Each component has a set of arguments (Alloy attributes and blocks) and
// optionally a set of exported fields. Components can reference the exports of
// other components using Alloy expressions.
//
// See the top-level component package for more information on components, and
// subpackages for defined components.
//
// # Component Health
//
// A component will have various health states during its lifetime:
//
//  1. Unknown:   The initial health state for new components.
//  2. Healthy:   A healthy component
//  3. Unhealthy: An unhealthy component.
//  4. Exited:    A component which is no longer running.
//
// Health states are paired with a time for when the health state was generated
// and a message providing more detail for the health state.
//
// Components can report their own health states. The health state reported by
// a component is merged with the Alloy-level health of that component: an error
// when evaluating the configuration for a component will always be reported as
// unhealthy until the next successful evaluation.
//
// # Node Evaluation
//
// The process of converting the Alloy block associated with a node into
// the appropriate Go struct is called "node evaluation."
//
// Nodes are only evaluated after all nodes they reference have been
// evaluated; cyclic dependencies are invalid.
//
// If a node updates its Exports at runtime, other nodes which directly
// or indirectly reference the updated node will have their Arguments
// re-evaluated.
//
// The arguments and exports for a node will be left in their last valid
// state if a node shuts down or is given an invalid config. This prevents
// a domino effect of a single failed node taking down other node
// which are otherwise healthy.
package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/worker"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
)

// Options holds static options for an Alloy controller.
type Options struct {
	// ControllerID is an identifier used to represent the controller.
	// ControllerID is used to generate a globally unique display name for
	// components in a binary where multiple controllers are used.
	//
	// If running multiple Alloy controllers, each controller must have a
	// different value for ControllerID to be able to differentiate between
	// components in telemetry data.
	ControllerID string

	// Logger to use for controller logs and components. A no-op logger will be
	// created if this is nil.
	Logger *logging.Logger

	// Tracer for components to use. A no-op tracer will be created if this is
	// nil.
	Tracer *tracing.Tracer

	// Directory where components can write data. Constructed components will be
	// given a subdirectory of DataPath using the local ID of the component.
	//
	// If running multiple Alloy controllers, each controller must have a
	// different value for DataPath to prevent components from colliding.
	DataPath string

	// Reg is the prometheus register to use
	Reg prometheus.Registerer

	// MinStability is the minimum stability level of features that can be used by the collector. It is defined by
	// the user, for example, via command-line flags.
	MinStability featuregate.Stability

	// OnExportsChange is called when the exports of the controller change.
	// Exports are controlled by "export" configuration blocks. If
	// OnExportsChange is nil, export configuration blocks are not allowed in the
	// loaded config source.
	OnExportsChange func(exports map[string]any)

	// List of Services to run with the Alloy controller.
	//
	// Services are configured when LoadFile is invoked. Services are started
	// when the Alloy controller runs after LoadFile is invoked at least once.
	Services []service.Service

	// EnableCommunityComps enables the use of community components.
	EnableCommunityComps bool
}

// Runtime is the Alloy system.
type Runtime struct {
	log    *logging.Logger
	tracer *tracing.Tracer
	opts   controllerOptions

	updateQueue *controller.Queue
	sched       *controller.Scheduler
	loader      *controller.Loader
	modules     *moduleRegistry

	loadFinished chan struct{}

	loadMut    sync.RWMutex
	loadedOnce atomic.Bool
}

// New creates a new, unstarted Alloy controller. Call Run to run the controller.
func New(o Options) *Runtime {
	return newController(controllerOptions{
		Options:        o,
		ModuleRegistry: newModuleRegistry(),
		IsModule:       false, // We are creating a new root controller.
		WorkerPool:     worker.NewDefaultWorkerPool(),
	})
}

// controllerOptions are internal options used to create both root Alloy
// controller and controllers for modules.
type controllerOptions struct {
	Options

	ComponentRegistry controller.ComponentRegistry // Custom component registry used in tests.
	ModuleRegistry    *moduleRegistry              // Where to register created modules.
	IsModule          bool                         // Whether this controller is for a module.
	// A worker pool to evaluate components asynchronously. A default one will be created if this is nil.
	WorkerPool worker.Pool
}

// newController creates a new, unstarted Alloy controller with a specific
// moduleRegistry. Modules created by the controller will be passed to the
// given modReg.
func newController(o controllerOptions) *Runtime {
	var (
		log        = o.Logger
		tracer     = o.Tracer
		workerPool = o.WorkerPool
	)

	if tracer == nil {
		var err error
		tracer, err = tracing.New(tracing.DefaultOptions)
		if err != nil {
			// This shouldn't happen unless there's a bug
			panic(err)
		}
	}

	if workerPool == nil {
		level.Info(log).Log("msg", "no worker pool provided, creating a default pool", "controller", o.ControllerID)
		workerPool = worker.NewDefaultWorkerPool()
	}

	f := &Runtime{
		log:    log,
		tracer: tracer,
		opts:   o,

		updateQueue: controller.NewQueue(),
		sched:       controller.NewScheduler(log),

		modules: o.ModuleRegistry,

		loadFinished: make(chan struct{}, 1),
	}

	serviceMap := controller.NewServiceMap(o.Services)

	f.loader = controller.NewLoader(controller.LoaderOptions{
		ComponentGlobals: controller.ComponentGlobals{
			Logger:               log,
			TraceProvider:        tracer,
			DataPath:             o.DataPath,
			MinStability:         o.MinStability,
			EnableCommunityComps: o.EnableCommunityComps,
			OnBlockNodeUpdate: func(cn controller.BlockNode) {
				// Changed node should be queued for reevaluation.
				f.updateQueue.Enqueue(&controller.QueuedNode{Node: cn, LastUpdatedTime: time.Now()})
			},
			OnExportsChange: o.OnExportsChange,
			Registerer:      o.Reg,
			ControllerID:    o.ControllerID,
			NewModuleController: func(id string) controller.ModuleController {
				return newModuleController(&moduleControllerOptions{
					ComponentRegistry:    o.ComponentRegistry,
					ModuleRegistry:       o.ModuleRegistry,
					Logger:               log,
					Tracer:               tracer,
					Reg:                  o.Reg,
					DataPath:             o.DataPath,
					MinStability:         o.MinStability,
					EnableCommunityComps: o.EnableCommunityComps,
					ID:                   id,
					ServiceMap:           serviceMap,
					WorkerPool:           workerPool,
				})
			},
			GetServiceData: func(name string) (interface{}, error) {
				svc, found := serviceMap.Get(name)
				if !found {
					return nil, fmt.Errorf("service %q does not exist", name)
				}
				return svc.Data(), nil
			},
		},

		Services:          o.Services,
		Host:              f,
		ComponentRegistry: o.ComponentRegistry,
		WorkerPool:        workerPool,
	})

	return f
}

// Run starts the Alloy controller, blocking until the provided context is
// canceled. Run must only be called once.
func (f *Runtime) Run(ctx context.Context) {
	defer func() { _ = f.sched.Close() }()
	defer f.loader.Cleanup(!f.opts.IsModule)
	defer level.Debug(f.log).Log("msg", "Alloy controller exiting")

	for {
		select {
		case <-ctx.Done():
			return

		case <-f.updateQueue.Chan():
			// Evaluate all nodes that have been updated. Sending the entire batch together will improve
			// throughput - it prevents the situation where two nodes have the same dependency, and the first time
			// it's picked up by the worker pool and the second time it's enqueued again, resulting in more evaluations.
			all := f.updateQueue.DequeueAll()
			f.loader.EvaluateDependants(ctx, all)
		case <-f.loadFinished:
			level.Info(f.log).Log("msg", "scheduling loaded components and services")

			var (
				components = f.loader.Components()
				services   = f.loader.Services()
				imports    = f.loader.Imports()

				runnables = make([]controller.RunnableNode, 0, len(components)+len(services)+len(imports))
			)
			for _, c := range components {
				runnables = append(runnables, c)
			}

			for _, i := range imports {
				runnables = append(runnables, i)
			}

			// Only the root controller should run services, since modules share the
			// same service instance as the root.
			if !f.opts.IsModule {
				for _, svc := range services {
					runnables = append(runnables, svc)
				}
			}

			err := f.sched.Synchronize(runnables)
			if err != nil {
				level.Error(f.log).Log("msg", "failed to load components and services", "err", err)
			}
		}
	}
}

// LoadSource synchronizes the state of the controller with the current config
// source. Components in the graph will be marked as unhealthy if there was an
// error encountered during Load.
//
// The controller will only start running components after Load is called once
// without any configuration errors.
// LoadSource uses default loader configuration.
func (f *Runtime) LoadSource(source *Source, args map[string]any) error {
	return f.loadSource(source, args, nil)
}

// Same as above but with a customComponentRegistry that provides custom component definitions.
func (f *Runtime) loadSource(source *Source, args map[string]any, customComponentRegistry *controller.CustomComponentRegistry) error {
	f.loadMut.Lock()
	defer f.loadMut.Unlock()

	applyOptions := controller.ApplyOptions{
		Args:                    args,
		ComponentBlocks:         source.components,
		ConfigBlocks:            source.configBlocks,
		DeclareBlocks:           source.declareBlocks,
		CustomComponentRegistry: customComponentRegistry,
	}

	diags := f.loader.Apply(applyOptions)
	if !f.loadedOnce.Load() && diags.HasErrors() {
		// The first call to Load should not run any components if there were
		// errors in the configuration file.
		return diags
	}
	f.loadedOnce.Store(true)

	select {
	case f.loadFinished <- struct{}{}:
	default:
		// A refresh is already scheduled
	}
	return diags.ErrorOrNil()
}

// Ready returns whether the Alloy controller has finished its initial load.
func (f *Runtime) Ready() bool {
	return f.loadedOnce.Load()
}
