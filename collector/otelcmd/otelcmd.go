// Package otelcmd provides the OpenTelemetry Collector command for Alloy.
// This package exports the collector functionality so it can be used from
// the main Alloy module.
package otelcmd

import (
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	envprovider "go.opentelemetry.io/collector/confmap/provider/envprovider"
	fileprovider "go.opentelemetry.io/collector/confmap/provider/fileprovider"
	httpprovider "go.opentelemetry.io/collector/confmap/provider/httpprovider"
	httpsprovider "go.opentelemetry.io/collector/confmap/provider/httpsprovider"
	yamlprovider "go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/otelcol"

	"github.com/grafana/alloy/otelcol/otelcmd/internal/components"
)

// NewCollectorSettings creates and returns the OTel Collector settings
// configured for Alloy.
func NewCollectorSettings() otelcol.CollectorSettings {
	info := component.BuildInfo{
		Command:     "alloy",
		Description: "Alloy OTel Collector distribution.",
		Version:     "v1.11.0",
	}

	return otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components.Components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				ProviderFactories: []confmap.ProviderFactory{
					envprovider.NewFactory(),
					fileprovider.NewFactory(),
					httpprovider.NewFactory(),
					httpsprovider.NewFactory(),
					yamlprovider.NewFactory(),
				},
			},
		},
		ProviderModules: map[string]string{
			envprovider.NewFactory().Create(confmap.ProviderSettings{}).Scheme():   "go.opentelemetry.io/collector/confmap/provider/envprovider v1.45.0",
			fileprovider.NewFactory().Create(confmap.ProviderSettings{}).Scheme():  "go.opentelemetry.io/collector/confmap/provider/fileprovider v1.45.0",
			httpprovider.NewFactory().Create(confmap.ProviderSettings{}).Scheme():  "go.opentelemetry.io/collector/confmap/provider/httpprovider v1.45.0",
			httpsprovider.NewFactory().Create(confmap.ProviderSettings{}).Scheme(): "go.opentelemetry.io/collector/confmap/provider/httpsprovider v1.45.0",
			yamlprovider.NewFactory().Create(confmap.ProviderSettings{}).Scheme():  "go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.45.0",
		},
		ConverterModules: []string{},
	}
}

// NewCollectorCommand creates a new Cobra command for the OTel Collector
// that integrates with Alloy's flowcmd.
func NewCollectorCommand(settings otelcol.CollectorSettings) *cobra.Command {
	otelCmd := otelcol.NewCommand(settings)
	// Modify the command to fit better in Alloy
	otelCmd.Use = "otel"
	otelCmd.Short = "Alloy OTel Collector runtime mode"
	otelCmd.Long = "Use Alloy with OpenTelemetry Collector runtime"
	return otelCmd
}
