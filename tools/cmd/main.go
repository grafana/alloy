package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/goversion"
	"github.com/grafana/alloy/tools/govulncheck"
)

func main() {
	cmd := &cobra.Command{
		Use: "tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(
		goversion.Command(),
		govulncheck.Command(),
	)

	if err := cmd.Execute(); err != nil {
		log.Fatalf("failed to run command: %v", err)
	}
}
