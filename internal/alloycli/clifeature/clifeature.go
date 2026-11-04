// Package clifeature lets optional parts of Alloy register features with the
// Alloy CLI, such as component tools exposed under `alloy tools` and the config
// converter. Features register from an init function, so they're only
// available when the package providing them is built into Alloy.
//
// This package must not import other Alloy packages with heavy dependencies,
// so that anything can register with it without pulling in the CLI.
package clifeature

import (
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/converter/diag"
)

// Tool is a group of command line utilities for a component.
type Tool struct {
	// Name of the component the tools are for, e.g. "prometheus.remote_write".
	Name string
	// Install adds the tools as subcommands of the provided command.
	Install func(cmd *cobra.Command)
}

// Converter converts config files from other formats into Alloy
// configurations.
type Converter struct {
	// Formats is the list of supported source formats.
	Formats []string
	// Convert converts in, which is in the given source format.
	Convert func(in []byte, sourceFormat string, extraArgs []string) ([]byte, diag.Diagnostics)
}

// Set is the set of features available to the CLI.
type Set struct {
	// Tools are exposed under `alloy tools`, sorted by name.
	Tools []Tool
	// Converter is nil if no converter is built into Alloy. Then `alloy convert`
	// is hidden and always returns an error, and run and validate hide the
	// --config.format, --config.bypass-conversion-errors and --config.extra-args
	// flags. The flags are still accepted, but any format other than "alloy"
	// returns an error.
	Converter *Converter
}

var (
	mut       sync.Mutex
	tools     []Tool
	converter *Converter
)

// RegisterTool registers tools for a component. It is intended to be called
// from an init function.
func RegisterTool(t Tool) {
	mut.Lock()
	defer mut.Unlock()
	tools = append(tools, t)
}

// RegisterConverter registers the config converter. It is intended to be
// called from an init function. If it's called more than once, the last
// converter registered is used.
func RegisterConverter(c Converter) {
	mut.Lock()
	defer mut.Unlock()
	converter = &c
}

// All returns all features registered with this package.
func All() Set {
	mut.Lock()
	defer mut.Unlock()
	res := slices.Clone(tools)
	slices.SortFunc(res, func(a, b Tool) int { return strings.Compare(a.Name, b.Name) })
	return Set{Tools: res, Converter: converter}
}
