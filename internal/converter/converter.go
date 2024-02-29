// Package converter exposes utilities to convert config files from other
// programs to Grafana Agent Flow configurations.
package converter

import (
	"fmt"

	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert"
	"github.com/grafana/agent/internal/converter/internal/promtailconvert"
	"github.com/grafana/agent/internal/converter/internal/staticconvert"
)

// Input represents the type of config file being fed into the converter.
type Input string

const (
	// InputPrometheus indicates that the input file is a prometheus YAML file.
	InputPrometheus Input = "prometheus"
	// InputPromtail indicates that the input file is a promtail YAML file.
	InputPromtail Input = "promtail"
	// InputStatic indicates that the input file is a grafana agent static YAML file.
	InputStatic Input = "static"
)

var SupportedFormats = []string{
	string(InputPrometheus),
	string(InputPromtail),
	string(InputStatic),
}

// Convert generates a Grafana Agent Flow config given an input configuration
// file.
//
// extraArgs are supported to be passed along to a converter such as enabling
// integrations-next for the static converter. Converters that do not support
// extraArgs will return a critical severity diagnostic if any are passed.
//
// Conversions are made as literally as possible, so the resulting config files
// may be unoptimized (i.e., lacking component reuse). A converted config file
// should just be the starting point rather than the final destination.
//
// Note that not all functionality defined in the input configuration may have
// an equivalent in Grafana Agent Flow. If the conversion could not complete
// because of mismatched functionality, an error is returned with no resulting
// config. If the conversion completed successfully but generated warnings, an
// error is returned alongside the resulting config.
func Convert(in []byte, kind Input, extraArgs []string) ([]byte, diag.Diagnostics) {
	switch kind {
	case InputPrometheus:
		return prometheusconvert.Convert(in, extraArgs)
	case InputPromtail:
		return promtailconvert.Convert(in, extraArgs)
	case InputStatic:
		return staticconvert.Convert(in, extraArgs)
	}

	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("unrecognized kind %q given to the config converter", kind))
	return nil, diags
}
