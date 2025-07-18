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
		HttpAddr:     "http://127.0.0.1:12345/graphql",
		MinStability: featuregate.StabilityGenerallyAvailable,
	}

	cmd := &cobra.Command{
		Use:          "repl [flags]",
		Short:        "Run Grafana Alloy REPL",
		Long:         "The repl subcommand allows for interactive diagnostics and data collection from a running Alloy instance.",
		Args:         cobra.NoArgs,
		Example:      "alloy repl",
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(`
╔═══════════════════════════════════════════════════════════════════════╗
║  This command is EXPERIMENTAL and may change or be removed in future  ║
║  versions. Use with caution in production environments.               ║
╚═══════════════════════════════════════════════════════════════════════╝

`)
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

	// Misc flags
	cmd.Flags().Var(&r.MinStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))

	return cmd
}
