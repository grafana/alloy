package main

import (
	"log"

	"github.com/grafana/alloy/tools/aireview"
	"github.com/grafana/alloy/tools/goversion"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use: "tooling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(
		aireview.Command(),
		goversion.Command(),
	)

	if err := cmd.Execute(); err != nil {
		log.Fatalf("failed to run command: %v", err)
	}
}
