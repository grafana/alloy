package util

import (
	"fmt"

	"go.opentelemetry.io/collector/featuregate"
	_ "go.opentelemetry.io/collector/obsreport"
)

var (
	// Enable the "telemetry.useOtelForInternalMetrics" Collector feature gate.
	// Currently, Collector components uses OpenCensus metrics by default.
	// Those metrics cannot be integrated with Alloy, so we need to always use
	// OpenTelemetry metrics.
	//
	// TODO: Remove "telemetry.useOtelForInternalMetrics" when Collector components
	//       use OpenTelemetry metrics by default.
	otelFeatureGates = []string{
		"telemetry.useOtelForInternalMetrics",
	}
)

// Enables a set of feature gates which should always be enabled in Alloy.
func SetupOtelFeatureGates() error {
	return EnableOtelFeatureGates(otelFeatureGates...)
}

// Enables a set of feature gates in Otel's Global Feature Gate Registry.
func EnableOtelFeatureGates(fgNames ...string) error {
	fgReg := featuregate.GlobalRegistry()

	for _, fg := range fgNames {
		err := fgReg.Set(fg, true)
		if err != nil {
			return fmt.Errorf("error setting Otel feature gate: %w", err)
		}
	}

	return nil
}
