// Package benchdiff compares Go benchmarks between two checkouts of the
// repository (typically a pull request's merge base and its head) and renders
// a Markdown report of the significant differences.
package benchdiff

import (
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchdiff",
		Short: "Compare Go benchmarks between two checkouts of the repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	cmd.AddCommand(
		packagesCommand(),
		runCommand(),
		reportCommand(),
	)

	return cmd
}
