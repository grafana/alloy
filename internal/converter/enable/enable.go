// Package enable registers the config converter with the Alloy CLI. Importing
// this package enables `alloy convert` and running Alloy with a non-Alloy
// --config.format.
package enable

import (
	"github.com/grafana/alloy/internal/alloycli"
	"github.com/grafana/alloy/internal/converter"
	"github.com/grafana/alloy/internal/converter/diag"
)

func init() {
	alloycli.RegisterConverter(func(in []byte, sourceFormat string, extraArgs []string) ([]byte, diag.Diagnostics) {
		return converter.Convert(in, converter.Input(sourceFormat), extraArgs)
	}, converter.SupportedFormats)
}
