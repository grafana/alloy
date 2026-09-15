package util

import (
	"fmt"

	"go.opentelemetry.io/collector/featuregate"

	// Registers the "k8sattr.fieldExtractConfigRegex.disallow" feature gate.
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
)

type gateDetails struct {
	name    string
	enabled bool
}

var (
	// filelog.allowFileDeletion was promoted from alpha to beta (enabled by default) upstream,
	// so it no longer needs to be set here. See otelFeatureGates below for gates still needed.
	otelFeatureGates = []gateDetails{}
)

// Enables a set of feature gates which should always be enabled in Alloy.
func SetupOtelFeatureGates() error {
	return EnableOtelFeatureGates(otelFeatureGates...)
}

// Enables a set of feature gates in Otel's Global Feature Gate Registry.
func EnableOtelFeatureGates(fgts ...gateDetails) error {
	fgReg := featuregate.GlobalRegistry()

	for _, fg := range fgts {
		err := fgReg.Set(fg.name, fg.enabled)
		if err != nil {
			return fmt.Errorf("error setting Otel feature gate: %w", err)
		}
	}

	return nil
}
