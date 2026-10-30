// Package alloyengine provides the alloyengine extension, which embeds Alloy's
// Default Engine in an OTel Collector distribution. It includes every native
// Alloy component, so it can be added to any OCB distribution.
package alloyengine

import (
	"go.opentelemetry.io/collector/extension"

	"github.com/grafana/alloy/extension/alloyengine/engine"

	// Include every native Alloy component.
	_ "github.com/grafana/alloy/internal/component/all"
)

// NewFactory creates a factory for the alloyengine extension.
func NewFactory() extension.Factory {
	return engine.NewFactory()
}
