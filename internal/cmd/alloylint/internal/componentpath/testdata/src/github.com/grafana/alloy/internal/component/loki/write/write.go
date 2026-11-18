package write

import "github.com/grafana/alloy/internal/component"

func init() {
	component.Register(component.Registration{Name: "loki.write"})
}
