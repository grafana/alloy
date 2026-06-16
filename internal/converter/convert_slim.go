//go:build slim

package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
)

// SupportedFormats is the reduced set of input formats this slim build can
// convert. otelcol and static conversion are excluded to drop their heavy
// dependency trees (OTel collector components, static-mode integrations).
var SupportedFormats = []string{
	string(InputPrometheus),
	string(InputPromtail),
}

func convertOtelcol(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("otelcol")
}

func convertStatic(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("static")
}

func unsupportedInSlim(format string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("%q conversion is not supported in this slim build of Alloy; use a full (non-slim) build to convert this format", format))
	return diags
}
