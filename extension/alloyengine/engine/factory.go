// Package engine implements the alloyengine extension without importing any
// native Alloy components or the config converter. Alloy's own OTel Collector
// distribution uses it so that the components selected in builder-config.yaml
// decide what is linked. Other OCB distributions should use the parent
// alloyengine package, which includes every component.
package engine

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	// typeStr is the type string for the alloyEngine extension.
	typeStr = component.MustNewType("alloyengine")

	// stability level of the component.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for the alloyengine extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		stability,
	)
}

// createDefaultConfig creates the default configuration for the extension.
func createDefaultConfig() component.Config {
	return &Config{}
}

// createExtension creates an alloyengine extension instance.
func createExtension(
	_ context.Context,
	settings extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {

	config := cfg.(*Config)

	return newAlloyEngineExtension(config, settings.TelemetrySettings), nil
}
