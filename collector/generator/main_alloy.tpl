package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/grafana/alloy/flowcmd"
	"github.com/grafana/alloy/internal/alloyseed"
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
	otelCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if !*disableReporting {
			startUsageReporter(cmd.Context())
		}
		return nil
	}

	flowCmd := flowcmd.RootCommand()
	flowCmd.AddCommand(otelCmd)

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

// startUsageReporter starts the anonymous usage stats reporter for the OTel Engine.
// The report is tagged engine="otel" (via useragent.GetEngineMode) and its metrics
// come from GlobalTracker, which holds the configured OTel component types and (when
// the alloyengine extension is used) the Alloy components it runs.
func startUsageReporter(ctx context.Context) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	alloyseed.Init("", logger)
	reporter, err := usagestats.NewReporter(logger)
	if err != nil {
		logger.Error("failed to create usage stats reporter", "err", err)
		return
	}
	go func() {
		if err := reporter.Start(ctx, usagestats.GlobalTracker.Metrics); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("failed to run usage stats reporter", "err", err)
		}
	}()
}
