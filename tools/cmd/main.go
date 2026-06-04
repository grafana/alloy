package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/aireview"
	"github.com/grafana/alloy/tools/generate"
	"github.com/grafana/alloy/tools/goversion"
	"github.com/grafana/alloy/tools/govulncheck"
	"github.com/grafana/alloy/tools/lint"
	"github.com/grafana/alloy/tools/release"
)

func main() {
	cmd := &cobra.Command{
		Use:          "tools",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(
		aireview.Command(),
		generate.Command(),
		goversion.Command(),
		govulncheck.Command(),
		release.Command(),
		lint.Command(),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
