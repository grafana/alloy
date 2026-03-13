package fanoutconsumer

import (
	"github.com/grafana/alloy/internal/component/otelcol"
)

// componentID returns the component ID of the consumer if it implements
// otelcol.ComponentMetadata, otherwise returns "undefined".
func componentID(c any) string {
	if m, ok := c.(otelcol.ComponentMetadata); ok {
		return m.ComponentID()
	}
	return "undefined"
}
