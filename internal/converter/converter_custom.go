//go:build alloy_custom_components

// Package converter exposes utilities to convert config files from other
// programs to Grafana Alloy configurations.
package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
)

// Input represents the type of config file being fed into the converter.
type Input string

const (
	// InputOtelCol indicates that the input file is an OpenTelemetry Collector YAML file.
	InputOtelCol Input = "otelcol"
	// InputPrometheus indicates that the input file is a prometheus YAML file.
	InputPrometheus Input = "prometheus"
	// InputPromtail indicates that the input file is a promtail YAML file.
	InputPromtail Input = "promtail"
	// InputStatic indicates that the input file is a grafana agent static YAML file.
	InputStatic Input = "static"
)

// SupportedFormats is empty in a custom component build. Configuration
// conversion retains many component implementations, so lightweight builds
// only accept Alloy configuration.
var SupportedFormats []string

// Convert reports that config conversion is unavailable in a custom component
// build. Native Alloy configuration is parsed directly and never reaches this
// function.
func Convert(_ []byte, kind Input, _ []string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	diags.Add(
		diag.SeverityLevelCritical,
		fmt.Sprintf("configuration converters are not included in this custom component build; cannot convert from %q; use Alloy configuration", kind),
	)
	return nil, diags
}
