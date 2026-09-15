//go:build !alloy_custom_components

package alloycli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultBuildRootCommands(t *testing.T) {
	cmd := Command()
	commandNames := make(map[string]struct{}, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		commandNames[child.Name()] = struct{}{}
	}

	for _, name := range []string{"convert", "fmt", "gql", "run", "tools", "validate"} {
		_, exists := commandNames[name]
		require.True(t, exists, "expected %q command", name)
	}
}
