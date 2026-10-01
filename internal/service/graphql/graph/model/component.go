package model

import "github.com/grafana/alloy/internal/component"

type Component struct {
	ID                   string         `json:"id"`
	ModuleID             string         `json:"moduleID"`
	Name                 string         `json:"name"`
	Label                *string        `json:"label,omitempty"`
	Type                 string         `json:"type"`
	Health               Health         `json:"health"`
	Edges                ComponentEdges `json:"edges"`
	LiveDebuggingEnabled bool           `json:"liveDebuggingEnabled"`
	Arguments            string         `json:"arguments"`
	Exports              string         `json:"exports"`
	DebugInfo            string         `json:"debugInfo"`

	// Internal component info used by field resolvers
	ComponentInfo *component.Info `json:"-"`
	RuntimeGraph  *RuntimeGraph   `json:"-"`
}

type ComponentEdges struct {
	Incoming []ComponentEdge `json:"incoming"`
	Outgoing []ComponentEdge `json:"outgoing"`
}

type ComponentEdge struct {
	SourceID     string        `json:"-"`
	ComponentID  string        `json:"componentID"`
	RuntimeGraph *RuntimeGraph `json:"-"`
}

func NewComponent(comp *component.Info) Component {
	var label *string
	if comp.Label != "" {
		label = &comp.Label
	}

	return Component{
		ID:       comp.ID.String(),
		ModuleID: comp.ID.ModuleID,
		Name:     comp.ComponentName,
		Label:    label,
		Type:     comp.Type.String(),
		Health: Health{
			State:       comp.Health.Health.String(),
			Message:     comp.Health.Message,
			LastUpdated: comp.Health.UpdateTime,
		},
		Edges:                NewComponentEdges(comp),
		LiveDebuggingEnabled: comp.LiveDebuggingEnabled,
		ComponentInfo:        comp,
	}
}

func NewComponentEdges(comp *component.Info) ComponentEdges {
	outgoing := make([]ComponentEdge, 0, len(comp.DataFlowEdgesTo))
	for _, id := range comp.DataFlowEdgesTo {
		outgoing = append(outgoing, ComponentEdge{
			SourceID:    comp.ID.String(),
			ComponentID: component.ID{ModuleID: comp.ID.ModuleID, LocalID: id}.String(),
		})
	}

	return ComponentEdges{
		Incoming: nil,
		Outgoing: outgoing,
	}
}
