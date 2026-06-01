package lint

import (
	"github.com/grafana/alloy/tools/lint/golint"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run linters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	cmd.AddCommand(
		golint.Command(),
	)

	return cmd
}
