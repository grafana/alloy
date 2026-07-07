package release

import (
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/release/backport"
	"github.com/grafana/alloy/tools/release/createrc"
	"github.com/grafana/alloy/tools/release/createreleasebranch"
	"github.com/grafana/alloy/tools/release/enrichreleasenotes"
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
		backport.Command(),
		createrc.Command(),
		createreleasebranch.Command(),
		enrichreleasenotes.Command(),
	)

	return cmd
}
