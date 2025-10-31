package alloyflow

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var (
	// typeStr is the type string for the alloyflow extension.
	typeStr = component.MustNewType("alloyflow")

	// stability level of the component.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for the alloyflow extension.
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

// createExtension creates an alloyflow extension instance.
func createExtension(
	_ context.Context,
	settings extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	config := cfg.(*Config)

	return newAlloyFlowExtension(config, settings.TelemetrySettings), nil
}
