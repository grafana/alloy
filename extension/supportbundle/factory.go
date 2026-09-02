package supportbundle

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
)

var (
	// typeStr is the type string for the supportbundle extension.
	typeStr = component.MustNewType("supportbundle")

	// stability level of the component.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for the supportbundle extension.
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
	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.NetAddr.Endpoint = "localhost:8089"
	// The handler holds the response open for the whole collection window, which
	// can exceed the confighttp default write timeout (30s). Disable it so the
	// download is not cut off; the handler is bounded by max_collection_duration.
	serverConfig.WriteTimeout = 0

	return &Config{
		ServerConfig:              serverConfig,
		Path:                      "/support",
		DefaultCollectionDuration: 30 * time.Second,
		MaxCollectionDuration:     60 * time.Second,
		// LogBufferSize defaults to 0: log capture is off unless the operator
		// sets a size, because it is always on and adds a per-line logging cost.
		//
		// Tracing is off by default. The sample count defaults to 10 (the zpages
		// default) so that enabling tracing alone works without setting it.
		Tracing: TracingConfig{SamplesPerSpan: 10},
	}
}

// createExtension creates a supportbundle extension instance.
func createExtension(
	_ context.Context,
	settings extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {

	config := cfg.(*Config)
	return newSupportBundleExtension(config, settings), nil
}
