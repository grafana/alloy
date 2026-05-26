package release

import "github.com/spf13/cobra"

const (
	releaseBranchPrefix = "release/v"
	backportLabelPrefix = "backport/v"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release automation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	cmd.AddCommand(
		backportCommand(),
		createReleaseBranchCommand(),
	)

	return cmd
}
