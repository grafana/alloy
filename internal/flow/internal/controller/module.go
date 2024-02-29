package controller

import (
	"context"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/river/ast"
)

// ModuleController is a lower-level interface for module controllers which
// allows probing for the list of managed modules.
type ModuleController interface {
	component.ModuleController

	// ModuleIDs returns the list of managed modules in unspecified order.
	ModuleIDs() []string

	// Creates a new custom component.
	NewCustomComponent(id string, export component.ExportFunc) (CustomComponent, error)
}

// CustomComponent is a controller for running components within a CustomComponent.
type CustomComponent interface {
	// LoadBody loads a River AST body into the CustomComponent. LoadBody can be called
	// multiple times, and called prior to [CustomComponent.Run].
	// customComponentRegistry provides custom component definitions for the loaded config.
	LoadBody(body ast.Body, args map[string]any, customComponentRegistry *CustomComponentRegistry) error

	// Run starts the CustomComponent. No components within the CustomComponent
	// will be run until Run is called.
	//
	// Run blocks until the provided context is canceled. The ID of a CustomComponent as defined in
	// ModuleController.NewCustomComponent will not be released until Run returns.
	Run(context.Context) error
}
