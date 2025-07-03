package alloycli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/alloycli/repl"
	"github.com/grafana/alloy/internal/featuregate"
)

func replCommand() *cobra.Command {
	r := &repl.AlloyRepl{
		HttpAddr: "http://127.0.0.1:12345/graphql",
		// storagePath:    "data-alloy/",
		MinStability: featuregate.StabilityGenerallyAvailable,
		// uiPrefix:     "/",
		// configFormat: "alloy",
	}

	cmd := &cobra.Command{
		Use:          "repl [flags]",
		Short:        "Run Grafana Alloy REPL",
		Long:         "The repl subcommand allows for interactive diagnostics and data collection from a running Alloy instance.",
		Args:         cobra.NoArgs,
		Example:      "alloy repl",
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			return r.Run(cmd)
		},
	}

	// Server flags
	cmd.Flags().
		StringVar(
			&r.HttpAddr,
			"server.graphql.endpoint",
			r.HttpAddr,
			"Address of the GraphQL endpoint",
		)
	// cmd.Flags().StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to discover the HTTP UI at")

	// Config flags
	// cmd.Flags().StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	// cmd.Flags().BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")
	// cmd.Flags().StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	// cmd.Flags().StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	cmd.Flags().Var(&r.MinStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	// cmd.Flags().BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")

	return cmd
}
