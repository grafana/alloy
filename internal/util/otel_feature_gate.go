package util

import (
	"fmt"

	"go.opentelemetry.io/collector/featuregate"

	// Register the feature gates.
	// The "service" package uses DisableHighCardinalityMetricsfeatureGate, so import "service".
	// We cannot import DisableHighCardinalityMetricsfeatureGate directly because it's not exported.
	_ "go.opentelemetry.io/collector/service"
)

var (
	otelFeatureGates = []string{}
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
