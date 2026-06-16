//go:build slim

package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
)

// SupportedFormats is empty in slim builds: config conversion is unavailable so
// that the converter dependency trees (OTel components, static-mode
// integrations, prometheus/promtail k8s service discovery) stay out of the
// binary.
var SupportedFormats = []string{}

func convertOtelcol(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("otelcol")
}
func convertPrometheus(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("prometheus")
}
func convertPromtail(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("promtail")
}
func convertStatic(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("static")
}

func unsupportedInSlim(format string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("%q conversion is not supported in this slim build of Alloy; use a full (non-slim) build to convert config", format))
	return diags
}
