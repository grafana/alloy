package alloycli

import (
	"errors"

	convert_diag "github.com/grafana/alloy/internal/converter/diag"
)

// ConvertFunc converts a config file of the given source format into an Alloy
// configuration.
type ConvertFunc func(in []byte, sourceFormat string, extraArgs []string) ([]byte, convert_diag.Diagnostics)

var (
	convertFn      ConvertFunc
	convertFormats []string
)

// errConverterNotIncluded is returned when config conversion is requested but
// no converter was registered. This happens when Alloy is built without
// converters.
var errConverterNotIncluded = errors.New("config conversion is not included in this build of Alloy")

// RegisterConverter registers the function used to convert config files from
// other formats into Alloy configurations, along with the list of supported
// source formats. It is intended to be called from an init function.
//
// Keeping the converter behind a registration hook allows it (and the
// components it depends on) to be left out of the binary at build time.
func RegisterConverter(fn ConvertFunc, formats []string) {
	convertFn = fn
	convertFormats = formats
}
