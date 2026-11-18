// Package benchdiff compares the Go benchmarks affected by a change between
// the change and its merge base.
package benchdiff

import (
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchdiff",
		Short: "Compare Go benchmarks between HEAD and its merge base",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(runCommand(), commentCommand())
	return cmd
}
