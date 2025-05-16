package runtime

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
)

// GetComponent implements [component.Provider].
func (f *Runtime) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	f.loadMut.RLock()
	defer f.loadMut.RUnlock()

	if id.ModuleID != "" {
		mod, ok := f.modules.Get(id.ModuleID)
		if !ok {
			return nil, component.ErrComponentNotFound
		}

		return mod.f.GetComponent(component.ID{LocalID: id.LocalID}, opts)
	}

	graph := f.loader.Graph()

	node := graph.GetByID(id.LocalID)
	if node == nil {
		return nil, component.ErrComponentNotFound
	}

	cn, ok := node.(controller.ComponentNode)
	if !ok {
		return nil, fmt.Errorf("%q is not a component", id)
	}

	return f.getComponentDetail(cn, graph, opts), nil
}

// ListComponents implements [component.Provider].
func (f *Runtime) ListComponents(moduleID string, opts component.InfoOptions) ([]*component.Info, error) {
	f.loadMut.RLock()
	defer f.loadMut.RUnlock()

	if moduleID != "" {
		mod, ok := f.modules.Get(moduleID)
		if !ok {
			return nil, component.ErrModuleNotFound
		}

		return mod.f.ListComponents("", opts)
	}

	var (
		components = f.loader.Components()
		graph      = f.loader.Graph()
	)

	detail := make([]*component.Info, len(components))
	for i, component := range components {
		detail[i] = f.getComponentDetail(component, graph, opts)
	}
	return detail, nil
}

func (f *Runtime) getComponentDetail(cn controller.ComponentNode, graph *dag.Graph, opts component.InfoOptions) *component.Info {
	var references, referencedBy []string

	// Skip over any edge which isn't between two component nodes. This is a
	// temporary workaround needed until there's a concept of configuration
	// blocks in the API.
	//
	// Without this change, the graph fails to render when a configuration
	// block is referenced in the graph.
	//
	// TODO(rfratto): add support for config block nodes in the API and UI.
	for _, dep := range graph.Dependencies(cn) {
		if _, ok := dep.(controller.ComponentNode); ok {
			references = append(references, dep.NodeID())
		}
	}
	for _, dep := range graph.Dependants(cn) {
		if _, ok := dep.(controller.ComponentNode); ok {
			referencedBy = append(referencedBy, dep.NodeID())
		}
	}

	// Fields which are optional to set.
	var (
		health    component.Health
		arguments component.Arguments
		exports   component.Exports
	)

	if opts.GetHealth {
		health = cn.CurrentHealth()
	}
	if opts.GetArguments {
		arguments = cn.Arguments()
	}
	if opts.GetExports {
		exports = cn.Exports()
	}

	componentInfo := &component.Info{
		ID: component.ID{
			ModuleID: f.opts.ControllerID,
			LocalID:  cn.NodeID(),
		},
		Label: cn.Label(),
		Type:  componentType(cn),

		References:   references,
		ReferencedBy: referencedBy,

		DataFlowEdgesTo: cn.GetDataFlowEdgesTo(),

		ComponentName: cn.ComponentName(),
		Health:        health,

		Arguments: arguments,
		Exports:   exports,

		ModuleIDs: cn.ModuleIDs(),
	}

	if builtinComponent, ok := cn.(*controller.BuiltinComponentNode); ok {
		componentInfo.Component = builtinComponent.Component()
		if opts.GetDebugInfo {
			componentInfo.DebugInfo = builtinComponent.DebugInfo()
		}
	}

	_, liveDebuggingEnabled := componentInfo.Component.(component.LiveDebugging)
	componentInfo.LiveDebuggingEnabled = liveDebuggingEnabled

	return componentInfo
}

func componentType(cn controller.ComponentNode) component.Type {
	if _, ok := cn.(*controller.BuiltinComponentNode); ok {
		return component.TypeBuiltin
	}

	return component.TypeCustom
}
