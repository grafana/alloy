package validator

import (
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/ast"
)

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
	parts := strings.Split(name, ".")
	// FIXME(kalleep): right now we register modules as custom components and only validate the
	// namespace part. We can't really know what components are contained within a module without
	// importing it first. Maybe we could have an option for validation to resolve modules so we could
	// validate them correctly but by default we don't resolve and validate them.
	if reg, ok := cr.custom[parts[0]]; ok {
		return reg, nil
	}

	return cr.parent.Get(name)
}

func (cr *componentRegistry) registerCustomComponent(c *ast.BlockStmt, args any) {
	// FIXME(kalleep): Figure out how to resolve args and exports for declares and how we could
	// support doing proper checks of modules.
	cr.custom[c.Label] = component.Registration{Name: c.Label, Args: args}
}
