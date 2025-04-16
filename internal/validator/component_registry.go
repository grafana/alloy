package validator

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/ast"
)

var _ component.Registry = (*componentRegistry)(nil)

func newComponentRegistry(cr component.Registry) *componentRegistry {
	return &componentRegistry{
		parent: cr,
		custom: make(map[string]component.Registration),
	}
}

// componentRegistry wraps a component.Registry and is used to register
// custom components (declare blocks).
type componentRegistry struct {
	parent component.Registry
	custom map[string]component.Registration
}

func (cr *componentRegistry) Get(name string) (component.Registration, error) {
	if reg, ok := cr.custom[name]; ok {
		return reg, nil
	}

	return cr.parent.Get(name)
}

func (cr *componentRegistry) registerCustomComponent(c *ast.BlockStmt) {
	// FIXME(kalleep): Figure out how to resolve args and exports for declares.
	cr.custom[c.Label] = component.Registration{Name: c.Label}
}
