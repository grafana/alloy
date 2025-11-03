// GENERATED CODE: DO NOT EDIT (the .go file, editing the .tpl file is okay)
package main

import (
	"github.com/grafana/alloy/flowcmd"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/otelcol"
)

func newAlloyCommand(params otelcol.CollectorSettings) *cobra.Command {
	otelCmd := otelcol.NewCommand(params)
	// Modify the command to fit better in Alloy
	otelCmd.Use = "otel"
	otelCmd.Short = "Alloy OTel Collector runtime mode"
	otelCmd.Long = "Use Alloy with OpenTelemetry Collector runtime"

	flowCmd := flowcmd.RootCommand()
	flowCmd.AddCommand(otelCmd)
	return flowCmd
}
