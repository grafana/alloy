package main

import (
	"os"

	"github.com/grafana/alloy/flowcmd"
	"github.com/grafana/alloy/otelcol/otelcmd"
)

func main() {
	flowCmd := flowcmd.RootCommand()

	// Add OTel Collector command
	otelSettings := otelcmd.NewCollectorSettings()
	otelCmd := otelcmd.NewCollectorCommand(otelSettings)
	flowCmd.AddCommand(otelCmd)

	if err := flowCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
