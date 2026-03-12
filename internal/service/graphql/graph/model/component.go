package model

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/encoding/alloyjson"
)

type Component struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Health    Health `json:"health"`
	Arguments string `json:"arguments"`
	Exports   string `json:"exports"`
	DebugInfo string `json:"debugInfo"`
}

func marshalBodyToString(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := alloyjson.MarshalBody(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func NewComponent(comp *component.Info) Component {
	return Component{
		ID:   comp.ID.String(),
		Name: comp.ComponentName,
		Health: Health{
			Message:     comp.Health.Message,
			LastUpdated: comp.Health.UpdateTime,
		},
		Arguments: marshalBodyToString(comp.Arguments),
		Exports:   marshalBodyToString(comp.Exports),
		DebugInfo: marshalBodyToString(comp.DebugInfo),
	}
}
