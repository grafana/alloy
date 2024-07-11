package runtime

import (
	"context"
	"fmt"
	"path"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/worker"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/scanner"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/maps"
)

type moduleController struct {
	mut     sync.RWMutex
	o       *moduleControllerOptions
	modules map[string]struct{}
}

var (
	_ component.ModuleController = (*moduleController)(nil)
)

// newModuleController is the entrypoint into creating module instances.
func newModuleController(o *moduleControllerOptions) controller.ModuleController {
	return &moduleController{
		o:       o,
		modules: map[string]struct{}{},
	}
}

// NewModule creates a new, unstarted Module.
func (m *moduleController) NewModule(id string, export component.ExportFunc) (component.Module, error) {
	if id != "" && !scanner.IsValidIdentifier(id) {
		return nil, fmt.Errorf("module ID %q is not a valid identifier", id)
	}

	m.mut.Lock()
	defer m.mut.Unlock()
	fullPath := m.o.ID
	if id != "" {
		fullPath = path.Join(fullPath, id)
	}

	mod := newModule(&moduleOptions{
		ID:                      fullPath,
		export:                  export,
		moduleControllerOptions: m.o,
		parent:                  m,
	})

	return mod, nil
}

// NewCustomComponent creates a new, unstarted CustomComponent.
func (m *moduleController) NewCustomComponent(id string, export component.ExportFunc) (controller.CustomComponent, error) {
	if id != "" && !scanner.IsValidIdentifier(id) {
		return nil, fmt.Errorf("customComponent ID %q is not a valid identifier", id)
	}

	m.mut.Lock()
	defer m.mut.Unlock()
	fullPath := m.o.ID
	if id != "" {
		fullPath = path.Join(fullPath, id)
	}

	mod := newModule(&moduleOptions{
		ID:                      fullPath,
		export:                  export,
		moduleControllerOptions: m.o,
		parent:                  m,
	})

	return mod, nil
}

func (m *moduleController) removeModule(mod *module) {
	m.mut.Lock()
	defer m.mut.Unlock()

	m.o.ModuleRegistry.Unregister(mod.o.ID)
	delete(m.modules, mod.o.ID)
}

func (m *moduleController) addModule(mod *module) error {
	m.mut.Lock()
	defer m.mut.Unlock()
	if err := m.o.ModuleRegistry.Register(mod.o.ID, mod); err != nil {
		level.Error(m.o.Logger).Log("msg", "error registering module", "id", mod.o.ID, "err", err)
		return err
	}
	m.modules[mod.o.ID] = struct{}{}
	return nil
}

// ModuleIDs implements [controller.ModuleController].
func (m *moduleController) ModuleIDs() []string {
	m.mut.RLock()
	defer m.mut.RUnlock()

	return maps.Keys(m.modules)
}

type module struct {
	f *Runtime
	o *moduleOptions
}

type moduleOptions struct {
	ID     string // ID is the full name including all parents, "module.file.example.prometheus.remote_write.id".
	export component.ExportFunc
	parent *moduleController
	*moduleControllerOptions
}

var (
	_ component.Module = (*module)(nil)
)

// newModule creates a module instance for a specific component.
func newModule(o *moduleOptions) *module {
	return &module{
		o: o,
		f: newController(controllerOptions{
			IsModule:          true,
			ModuleRegistry:    o.ModuleRegistry,
			ComponentRegistry: o.ComponentRegistry,
			WorkerPool:        o.WorkerPool,
			Options: Options{
				ControllerID:         o.ID,
				Tracer:               o.Tracer,
				Reg:                  o.Reg,
				Logger:               o.Logger,
				DataPath:             o.DataPath,
				MinStability:         o.MinStability,
				EnableCommunityComps: o.EnableCommunityComps,
				OnExportsChange: func(exports map[string]any) {
					if o.export != nil {
						o.export(exports)
					}
				},
				Services: o.ServiceMap.List(),
			},
		}),
	}
}

// LoadConfig parses Alloy config and loads it.
func (c *module) LoadConfig(config []byte, args map[string]any) error {
	ff, err := ParseSource(c.o.ID, config)
	if err != nil {
		return err
	}
	return c.f.LoadSource(ff, args)
}

// LoadBody loads a pre-parsed Alloy config.
func (c *module) LoadBody(body ast.Body, args map[string]any, customComponentRegistry *controller.CustomComponentRegistry) error {
	ff, err := sourceFromBody(body)
	if err != nil {
		return err
	}
	return c.f.loadSource(ff, args, customComponentRegistry)
}

// Run starts the Module. No components within the Module
// will be run until Run is called.
//
// Run blocks until the provided context is canceled.
func (c *module) Run(ctx context.Context) error {
	if err := c.o.parent.addModule(c); err != nil {
		return err
	}
	defer c.o.parent.removeModule(c)

	c.f.Run(ctx)
	return nil
}

// moduleControllerOptions holds static options for module controller.
type moduleControllerOptions struct {
	// Logger to use for controller logs and components. A no-op logger will be
	// created if this is nil.
	Logger *logging.Logger

	// Tracer for components to use. A no-op tracer will be created if this is
	// nil.
	Tracer *tracing.Tracer

	// Reg is the prometheus register to use
	Reg prometheus.Registerer

	// A path to a directory with this component may use for storage. The path is
	// guaranteed to be unique across all running components.
	//
	// The directory may not exist when the component is created; components
	// should create the directory if needed.
	DataPath string

	// MinStability is the minimum stability level of features that can be used by the collector. It is defined by
	// the user, for example, via command-line flags.
	MinStability featuregate.Stability

	// ID is the attached components full ID.
	ID string

	// ComponentRegistry is where controllers can look up components.
	ComponentRegistry controller.ComponentRegistry

	// ModuleRegistry is a shared registry of running modules from the same root
	// controller.
	ModuleRegistry *moduleRegistry

	// ServiceMap is a map of services which can be used in the module
	// controller.
	ServiceMap controller.ServiceMap

	// WorkerPool is a worker pool that can be used to run tasks asynchronously. A default pool will be created if this
	// is nil.
	WorkerPool worker.Pool

	// EnableCommunityComps enables the use of community components.
	EnableCommunityComps bool
}
