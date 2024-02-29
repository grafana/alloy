// Package flowmode is the entrypoint for Grafana Agent Flow.
package flowmode

import (
	"fmt"
	"os"

	"github.com/grafana/agent/internal/build"
	"github.com/spf13/cobra"
)

// Run is the entrypoint to Flow mode. It is expected to be called
// directly from the main function.
func Run() {
	var cmd = &cobra.Command{
		Use:     fmt.Sprintf("%s [global options] <subcommand>", os.Args[0]),
		Short:   "Grafana Agent Flow",
		Version: build.Print("agent"),

		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.SetVersionTemplate("{{ .Version }}\n")

	cmd.AddCommand(
		convertCommand(),
		fmtCommand(),
		runCommand(),
		toolsCommand(),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
