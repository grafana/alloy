package model

import "github.com/grafana/alloy/internal/component"

type Component struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Health Health `json:"health"`
}

func NewComponent(comp *component.Info) Component {
	return Component{
		ID:   comp.ID.String(),
		Name: comp.ComponentName,
		Health: Health{
			Message:     comp.Health.Message,
			LastUpdated: comp.Health.UpdateTime,
		},
	}
}
