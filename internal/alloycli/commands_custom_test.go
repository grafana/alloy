//go:build alloy_custom_components

package alloycli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomBuildRootCommands(t *testing.T) {
	cmd := Command()
	commandNames := make(map[string]struct{}, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		commandNames[child.Name()] = struct{}{}
	}

	for _, name := range []string{"run", "validate", "fmt", "gql"} {
		_, exists := commandNames[name]
		require.True(t, exists, "expected %q command", name)
	}
	for _, name := range []string{"convert", "tools"} {
		_, exists := commandNames[name]
		require.False(t, exists, "did not expect %q command", name)
	}

	run, _, err := cmd.Find([]string{"run"})
	require.NoError(t, err)
	require.Contains(t, run.Flag("config.format").Usage, `"alloy"`)
}
