package util

import (
	"fmt"

	"go.opentelemetry.io/collector/featuregate"
	_ "go.opentelemetry.io/collector/obsreport"
)

var (
	// Enable the "telemetry.useOtelForInternalMetrics" Collector feature gate.
	// Currently, Collector components uses OpenCensus metrics by default.
	// Those metrics cannot be integrated with Agent Flow,
	// so we need to always use OpenTelemetry metrics.
	//
	// TODO: Remove "telemetry.useOtelForInternalMetrics" when Collector components
	//       use OpenTelemetry metrics by default.
	flowModeOtelFeatureGates = []string{
		"telemetry.useOtelForInternalMetrics",
	}
)

// Enables a set of feature gates which should always be enabled for Flow mode.
func SetupFlowModeOtelFeatureGates() error {
	return EnableOtelFeatureGates(flowModeOtelFeatureGates...)
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
