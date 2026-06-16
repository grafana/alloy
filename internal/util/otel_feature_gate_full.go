//go:build !slim

package util

import (
	// Registers the "k8sattr.fieldExtractConfigRegex.disallow" feature gate.
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	// Registers the "filelog.allowFileDeletion" feature gate.
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

// otelFeatureGates are enabled by SetupOtelFeatureGates in the full build.
var otelFeatureGates = []gateDetails{
	{
		// This feature gate allows users of the otel filelogreceiver to use the `delete_after_read` setting.
		name:    "filelog.allowFileDeletion",
		enabled: true,
	},
}
