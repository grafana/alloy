//go:build !slim

package converter

import (
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/promtailconvert"
	"github.com/grafana/alloy/internal/converter/internal/staticconvert"
)

// SupportedFormats is the full set of input formats this build can convert.
var SupportedFormats = []string{
	string(InputOtelCol),
	string(InputPrometheus),
	string(InputPromtail),
	string(InputStatic),
}

func convertOtelcol(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return otelcolconvert.Convert(in, extraArgs)
}

func convertPrometheus(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return prometheusconvert.Convert(in, extraArgs)
}

func convertPromtail(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return promtailconvert.Convert(in, extraArgs)
}

func convertStatic(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return staticconvert.Convert(in, extraArgs)
}
