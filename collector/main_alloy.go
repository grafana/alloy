// GENERATED CODE: DO NOT EDIT
package main

import (
	"github.com/grafana/alloy/flowcmd"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/otelcol"
)

func newAlloyCommand(params otelcol.CollectorSettings) *cobra.Command {
    otelCmd := otelcol.NewCommand(params)

    otelCmd.Use = "otel"
    otelCmd.Short = "Use Alloy with OTel Engine"
    otelCmd.Long = "[EXPERIMENTAL] Use Alloy with OpenTelemetry Collector Engine"

    flowCmd := flowcmd.RootCommand()
    flowCmd.AddCommand(otelCmd)

    return flowCmd
}