package write

import "github.com/grafana/alloy/internal/component"

// Test files aren't checked.
func init() {
	component.Register(component.Registration{Name: "test.only"})
}
