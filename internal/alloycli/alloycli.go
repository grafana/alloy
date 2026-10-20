// Package alloycli is the entrypoint for Grafana Alloy.
package alloycli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/alloycli/clifeature"
	"github.com/grafana/alloy/internal/build"
)

// Command returns the root command with the features registered with
// [clifeature].
func Command() *cobra.Command {
	return NewCommand(clifeature.Default())
}

// NewCommand returns the root command with the given features.
func NewCommand(features clifeature.Set) *cobra.Command {
	var cmd = &cobra.Command{
		Use:     fmt.Sprintf("%s [global options] <subcommand>", filepath.Base(os.Args[0])),
		Short:   "Grafana Alloy",
		Long:    withConverterNote(features.Converter, "Grafana Alloy", noConverterNote),
		Version: build.Print("alloy"),

		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.SetVersionTemplate("{{ .Version }}\n")

	cmd.AddCommand(
		convertCommand(features.Converter),
		fmtCommand(),
		gqlCommand(),
		runCommand(features.Converter),
		toolsCommand(features.Tools),
		validateCommand(features.Converter),
	)

	return cmd
}
