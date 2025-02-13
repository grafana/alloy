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
	otelFeatureGates = []gateDetails{
		{
			// We're setting this feature gate since we don't yet know whether the
			// feature it deprecates will be removed.
			//TODO: Remove this once the feature gate in the Collector is "deprecated".
			name:    "k8sattr.fieldExtractConfigRegex.disallow",
			enabled: false,
		},
		{
			// This feature gate allows users of the otel filelogreceiver to use the `delete_after_read` setting.
			name:    "filelog.allowFileDeletion",
			enabled: true,
		},
		{
			// This feature gate allows users of the otel filelogreceiver to use the `header` setting.
			name:    "filelog.allowHeaderMetadataParsing",
			enabled: true,
		},
	}
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
