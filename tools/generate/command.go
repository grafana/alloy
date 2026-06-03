package generate

import (
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/generate/moduledeps"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generators for derived files in the Alloy repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	cmd.AddCommand(
		moduledeps.Command(),
	)

	return cmd
}
