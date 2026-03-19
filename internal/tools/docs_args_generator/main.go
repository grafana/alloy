package main

import (
	"github.com/spf13/cobra"
)

func main() {
	cmd := NewCommand()
	cobra.CheckErr(cmd.Execute())
}

func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "docs_args_generator",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			docsBase := ""
			if len(args) >= 3 {
				docsBase = args[2]
			}
			return generate(args[0], args[1], docsBase)
		},
	}

	return rootCmd
}
