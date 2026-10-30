package write

import "github.com/grafana/alloy/internal/component"

// Platform-specific files may register the same component.
func init() {
	component.Register(component.Registration{Name: "loki.write"})
}
