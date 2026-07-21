package main

import (
	"github.com/grafana/alloy/flowcmd"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/otelcol"
)

func newAlloyCommand(params otelcol.CollectorSettings) *cobra.Command {
    otelCmd := otelcol.NewCommand(params)

    otelCmd.Use = useragent.EngineOTel
    otelCmd.Short = "Use Alloy with OTel Engine"
    otelCmd.Long = "[EXPERIMENTAL] Use Alloy with OpenTelemetry Collector Engine"

    flowCmd := flowcmd.RootCommand()
    flowCmd.AddCommand(otelCmd)

    return flowCmd
}
