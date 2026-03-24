package controller

import (
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/ast"
)

// ComponentNodeManager is responsible for creating new component nodes and
// obtaining the necessary information to run them.
type ComponentNodeManager struct {
	globals ComponentGlobals
	// builtinComponentReg returns information to build and run built-in components.
	builtinComponentReg component.Registry

	mut sync.RWMutex
	// customComponentReg returns information to build and run custom components.
	customComponentReg *CustomComponentRegistry
}

// NewComponentNodeManager creates a new ComponentNodeManager without custom component registry.
func NewComponentNodeManager(globals ComponentGlobals, componentReg component.Registry) *ComponentNodeManager {
	return &ComponentNodeManager{
		globals:             globals,
		builtinComponentReg: componentReg,
	}
}

// CreateComponentNode creates a new builtin component or a new custom component.
func (m *ComponentNodeManager) createComponentNode(componentName string, block *ast.BlockStmt) (ComponentNode, error) {
	if isCustomComponent(m.customComponentReg, block.Name[0]) {
		return NewCustomComponentNode(m.globals, block, m.getCustomComponentConfig), nil
	}
	registration, err := m.builtinComponentReg.Get(componentName)
	if err != nil {
		return nil, err
	}
	if block.Label == "" {
		return nil, fmt.Errorf("component %q must have a label", componentName)
	}
	return NewBuiltinComponentNode(m.globals, registration, block), nil
}

// getCustomComponentConfig is used by the custom component to retrieve its template and the customComponentRegistry associated with it.
func (m *ComponentNodeManager) getCustomComponentConfig(namespace string, componentName string) (ast.Body, *CustomComponentRegistry, error) {
	m.mut.Lock()
	defer m.mut.Unlock()

	var (
		template                ast.Body
		customComponentRegistry *CustomComponentRegistry
	)

	if namespace == "" {
		template, customComponentRegistry = findLocalDeclare(m.customComponentReg, componentName)
	} else {
		template, customComponentRegistry = findImportedDeclare(m.customComponentReg, namespace, componentName)
	}

	if customComponentRegistry == nil || template == nil {
		return nil, nil, fmt.Errorf("custom component config not found in the registry, namespace: %q, componentName: %q", namespace, componentName)
	}
	// The registry is passed as a pointer to the custom component config.
	return template, customComponentRegistry, nil
}

// isCustomComponent returns true if the name matches a declare in the provided custom component registry.
func isCustomComponent(reg *CustomComponentRegistry, name string) bool {
	if reg == nil {
		return false
	}
	_, declareExists := reg.getDeclare(name)
	_, importExists := reg.getImport(name)
	return declareExists || importExists || isCustomComponent(reg.parent, name)
}

// findLocalDeclare recursively searches for a declare definition in the custom component registry.
func findLocalDeclare(reg *CustomComponentRegistry, componentName string) (ast.Body, *CustomComponentRegistry) {
	if declare, ok := reg.getDeclare(componentName); ok {
		return declare, reg
	}
	if reg.parent != nil {
		return findLocalDeclare(reg.parent, componentName)
	}
	return nil, nil
}

// findImportedDeclare recursively searches for an import matching the provided namespace.
// When the import is found, it will search for a declare matching the componentName within the custom registry of the import.
func findImportedDeclare(reg *CustomComponentRegistry, namespace string, componentName string) (ast.Body, *CustomComponentRegistry) {
	if imported, ok := reg.getImport(namespace); ok {
		if declare, ok := imported.getDeclare(componentName); ok {
			return declare, imported
		}
	}
	if reg.parent != nil {
		return findImportedDeclare(reg.parent, namespace, componentName)
	}
	return nil, nil
}

func (m *ComponentNodeManager) setCustomComponentRegistry(reg *CustomComponentRegistry) {
	m.mut.Lock()
	defer m.mut.Unlock()

	m.customComponentReg = reg
}
