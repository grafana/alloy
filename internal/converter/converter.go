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

// Convert generates a Grafana Alloy config given an input configuration file.
//
// All format-specific conversion is delegated to build-tag-gated helpers
// (see convert_heavy.go / convert_slim.go). slim builds support no conversion
// formats, which keeps their heavy dependency trees out of the binary.
func Convert(in []byte, kind Input, extraArgs []string) ([]byte, diag.Diagnostics) {
	switch kind {
	case InputOtelCol:
		return convertOtelcol(in, extraArgs)
	case InputPrometheus:
		return convertPrometheus(in, extraArgs)
	case InputPromtail:
		return convertPromtail(in, extraArgs)
	case InputStatic:
		return convertStatic(in, extraArgs)
	}

	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("unrecognized kind %q given to the config converter", kind))
	return nil, diags
}
