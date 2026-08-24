package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/grafana/alloy/flowcmd"
	"github.com/grafana/alloy/internal/usagestats"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/otelcol"
)

func newAlloyCommand(params otelcol.CollectorSettings) *cobra.Command {
	// Record the configured OTel Collector component types at config-load time so
	// they can be included in the anonymous usage stats report.
	params.ConfigProviderSettings.ResolverSettings.ConverterFactories = append(
		params.ConfigProviderSettings.ResolverSettings.ConverterFactories,
		confmap.NewConverterFactory(func(confmap.ConverterSettings) confmap.Converter {
			return usageStatsConverter{}
		}),
	)

	otelCmd := otelcol.NewCommand(params)

	otelCmd.Use = useragent.EngineOTel
	otelCmd.Short = "Use Alloy with OTel Engine"
	otelCmd.Long = "[EXPERIMENTAL] Use Alloy with OpenTelemetry Collector Engine"

	// Match `alloy run`: report anonymous usage stats unless the user opts out.
	disableReporting := otelCmd.Flags().Bool("disable-reporting", false, "Disable reporting of enabled components to Grafana.")

	// Start the usage stats reporter alongside the collector. PreRunE runs after
	// flags are parsed and only when the collector actually runs, not for
	// subcommands such as validate/components/print-config or --help.
	prevPreRunE := otelCmd.PreRunE
	otelCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if prevPreRunE != nil {
			if err := prevPreRunE(cmd, args); err != nil {
				return err
			}
		}
		if !*disableReporting {
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			// Use an empty seedDir since the OTel Engine cmd line has no storage path flag.
			// The seed will fall back to the platform default location.
			usagestats.StartReporter(cmd.Context(), logger, "", usagestats.GlobalTracker)
		}
		return nil
	}

	flowCmd := flowcmd.RootCommand()
	flowCmd.AddCommand(otelCmd)
	flowCmd.AddCommand(newOtelSupervisorCommand())

	return flowCmd
}

// usageStatsConverter records the configured OTel Collector component types into
// the process-wide usage stats tracker. It runs on every config resolution.
type usageStatsConverter struct{}

func (usageStatsConverter) Convert(_ context.Context, conf *confmap.Conf) error {
	components := usagestats.ExtractOtelComponents(conf.ToStringMap())
	usagestats.GlobalTracker.SetOTelComponentsFunc(func() map[string][]string { return components })
	return nil
}
