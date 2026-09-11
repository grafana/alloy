//go:build !alloy_custom_components

package alloycli

import "github.com/spf13/cobra"

func addRootCommands(cmd *cobra.Command) {
	cmd.AddCommand(
		convertCommand(),
		fmtCommand(),
		gqlCommand(),
		RunCommand(),
		toolsCommand(),
		validateCommand(),
	)
}
