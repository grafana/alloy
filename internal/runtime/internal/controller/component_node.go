package controller

import (
	"github.com/grafana/alloy/internal/component"
)

// ComponentNode is a generic representation of a component.
type ComponentNode interface {
	RunnableNode

	// CurrentHealth returns the current health of the component.
	CurrentHealth() component.Health

	// Arguments returns the current arguments of the managed component.
	Arguments() component.Arguments

	// Exports returns the current set of exports from the managed component.
	Exports() component.Exports

	// Label returns the component label.
	Label() string

	// ComponentName returns the name of the component.
	ComponentName() string

	// ID returns the component ID of the managed component from its Alloy block.
	ID() ComponentID

	// ModuleIDs returns the current list of modules managed by the component.
	ModuleIDs() []string

	// ResetDataFlowEdgeTo resets the current list of outgoing data flow edges.
	ResetDataFlowEdgeTo()

	// AddDataFlowEdgeTo adds an outgoing data flow edge to the component.
	AddDataFlowEdgeTo(nodeID string)

	// GetDataFlowEdgesTo returns the current list of outgoing data flow edges.
	GetDataFlowEdgesTo() []string
}
