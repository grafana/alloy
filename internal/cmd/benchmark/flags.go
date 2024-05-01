package main

import (
	"fmt"
	"os"

	"github.com/grafana/alloy/internal/build"
	"github.com/spf13/cobra"
)

func flags() {

	var cmd = &cobra.Command{
		Use:     fmt.Sprintf("%s [global options] <subcommand>", os.Args[0]),
		Short:   "Grafana Alloy Benchmark",
		Version: build.Print("benchmark"),

		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.SetVersionTemplate("{{ .Version }}\n")

	cmd.AddCommand(
		metricsCommand(),
		logsCommand(),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}

}
