// Package alloyengine provides the alloyengine extension, which embeds Alloy's
// Default Engine in an OTel Collector distribution. It includes every native
// Alloy component and the config converter, so it can be added to any OCB
// distribution.
package alloyengine

import (
	"go.opentelemetry.io/collector/extension"

	"github.com/grafana/alloy/extension/alloyengine/engine"

	// Include every native Alloy component and the config converter.
	_ "github.com/grafana/alloy/internal/component/all"
	_ "github.com/grafana/alloy/internal/converter/enable"
)

type (
	// Config is the configuration of the alloyengine extension.
	Config = engine.Config
	// AlloyConfig is the Alloy configuration source of the extension.
	AlloyConfig = engine.AlloyConfig
	// InlineAlloyConfig is an Alloy configuration set inline.
	InlineAlloyConfig = engine.InlineAlloyConfig
)

// NewFactory creates a factory for the alloyengine extension.
func NewFactory() extension.Factory {
	return engine.NewFactory()
}
