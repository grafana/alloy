package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// Execute runs the root command.
func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:               "generate-collector",
		Short:             "Generate the Alloy OTel Collector distribution (components, main, go.mod).",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.AddCommand(newGenerateCommand())
	return root
}
