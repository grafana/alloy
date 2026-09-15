//go:build alloy_custom_components

package alloycli

import "github.com/spf13/cobra"

func addRootCommands(cmd *cobra.Command) {
	cmd.AddCommand(
		fmtCommand(),
		gqlCommand(),
		RunCommand(),
		validateCommand(),
	)
}
