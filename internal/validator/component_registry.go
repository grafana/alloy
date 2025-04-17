package validator

import (
	"strings"

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
	parts := strings.Split(name, ".")
	// Custom component registered with their label and should be the first part of the name.
	// This does hower.
	// FIXME(kalleep): right now we register modules as a custom component and only validate the
	// the namespace part. We cant really know what componants is contained within a module without
	// importing in first. Maybe we could have a option for validation to resolve modules so we could
	// validate them correctly but by default we don't resolve and validate them.
	if reg, ok := cr.custom[parts[0]]; ok {
		return reg, nil
	}

	return cr.parent.Get(name)
}

func (cr *componentRegistry) registerCustomComponent(c *ast.BlockStmt) {
	// FIXME(kalleep): Figure out how to resolve args and exports for declares and how we could
	// support doing proper checks of modules.
	cr.custom[c.Label] = component.Registration{Name: c.Label}
}
