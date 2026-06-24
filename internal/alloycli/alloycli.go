// Package alloycli is the entrypoint for Grafana Alloy.
package alloycli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/build"
)

func Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     fmt.Sprintf("%s [global options] <subcommand>", filepath.Base(os.Args[0])),
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
		gqlCommand(),
		RunCommand(),
		securityPolicyCommand(),
		toolsCommand(),
		validateCommand(),
	)

	return cmd
}
