// Package alloycli is the entrypoint for Grafana Alloy.
package alloycli

import (
	"fmt"
	"os"

	"github.com/grafana/alloy/internal/build"
	"github.com/spf13/cobra"
)

// Run runs the Alloy CLI. It is expected to be called directly from the main
// function.
func Run() {
	var cmd = &cobra.Command{
		Use:     fmt.Sprintf("%s [global options] <subcommand>", os.Args[0]),
		Short:   "Grafana Alloy",
		Version: build.Print("alloy"),

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
		validateCommand(),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
