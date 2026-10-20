// Package clitools lets components register command line utilities which are
// exposed under `alloy tools`. Components register their tools from an init
// function, so tools are only available when the component is built into
// Alloy.
package clitools

import (
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// Tool is a group of command line utilities for a component.
type Tool struct {
	// Name of the component the tools are for, e.g. "prometheus.remote_write".
	Name string
	// Install adds the tools as subcommands of the provided command.
	Install func(cmd *cobra.Command)
}

var (
	mut   sync.Mutex
	tools []Tool
)

// Register registers tools for a component. It is intended to be called from
// an init function.
func Register(t Tool) {
	mut.Lock()
	defer mut.Unlock()
	tools = append(tools, t)
}

// All returns all registered tools, sorted by name.
func All() []Tool {
	mut.Lock()
	defer mut.Unlock()
	res := slices.Clone(tools)
	slices.SortFunc(res, func(a, b Tool) int { return strings.Compare(a.Name, b.Name) })
	return res
}
