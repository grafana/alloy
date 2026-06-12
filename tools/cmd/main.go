package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/aireview"
	"github.com/grafana/alloy/tools/goversion"
	"github.com/grafana/alloy/tools/govulncheck"
	"github.com/grafana/alloy/tools/lint"
	"github.com/grafana/alloy/tools/release"
	"github.com/grafana/alloy/tools/sync-replaces"
)

func main() {
	cmd := newRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tools",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(
		aireview.Command(),
		goversion.Command(),
		govulncheck.Command(),
		release.Command(),
		lint.Command(),
		syncreplaces.Command(),
	)

	return cmd
}
