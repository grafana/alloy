package main

import (
	"github.com/spf13/cobra"
)

func main() {
	cmd, err := NewCommand()
	cobra.CheckErr(err)
	cobra.CheckErr(cmd.Execute())
}

func NewCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:          "docs_args_generator",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generate(args[0], args[1])
		},
	}

	return rootCmd, nil
}
