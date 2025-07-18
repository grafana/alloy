package alloycli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/graphql/client"
	"github.com/grafana/alloy/internal/service/graphql/utils"
)

type alloyGql struct {
	HttpAddr     string
	MinStability featuregate.Stability
}

func gqlCommand() *cobra.Command {
	g := &alloyGql{
		HttpAddr:     "http://127.0.0.1:12345/graphql",
		MinStability: featuregate.StabilityGenerallyAvailable,
	}

	cmd := &cobra.Command{
		Use:   "gql <query>",
		Short: "Runs a GraphQL query against the Alloy instance",
		Long: `The gql subcommand runs a GraphQL query against the Alloy instance.

The query is provided as a single argument to the command.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Aliases:      []string{"graphql"},

		RunE: func(_ *cobra.Command, args []string) error {
			return g.Run(args[0])
		},
	}

	cmd.Flags().
		StringVar(
			&g.HttpAddr,
			"server.graphql.endpoint",
			g.HttpAddr,
			"Address of the GraphQL endpoint",
		)

	cmd.Flags().Var(
		&g.MinStability,
		"stability.level",
		fmt.Sprintf(
			"Minimum stability level of features to enable. Supported values: %s",
			strings.Join(featuregate.AllowedValues(), ", "),
		),
	)

	return cmd
}

func (g *alloyGql) Run(query string) error {
	client := client.NewGraphQlClient(g.HttpAddr)

	// Wrap in query {...} for convenience. If we ever add mutations, this will need to be updated.
	if !strings.HasPrefix(query, "query") {
		query = "query { " + query + " }"
	}

	response, err := client.Execute(query)
	if err != nil {
		fmt.Println("Error executing query. Is Alloy running?")
		return err
	}

	utils.PrintGraphQlResponse(response)

	return nil
}
