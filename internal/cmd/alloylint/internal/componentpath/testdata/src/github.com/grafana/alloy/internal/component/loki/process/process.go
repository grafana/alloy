package process

import "github.com/grafana/alloy/internal/component"

// Constant names are supported.
const name = "loki." + "process"

func init() {
	component.Register(component.Registration{Name: name})
}
