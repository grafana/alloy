package alloycli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/service/graphql/client"
	"github.com/grafana/alloy/internal/service/graphql/utils"
)

type alloyGql struct {
	httpAddr string
}

func gqlCommand() *cobra.Command {
	g := &alloyGql{
		httpAddr: "http://127.0.0.1:12345/graphql",
	}

	cmd := &cobra.Command{
		Use:   "gql <query>",
		Short: "[EXPERIMENTAL] Run a GraphQL query against a running Alloy instance",
		Long: `The gql subcommand runs a GraphQL query against a running Alloy instance.
The query is provided as a single argument to the command.

It requires the --feature.graphql.enabled flag on the running Alloy instance to
be set, as well as the stability.level flag set to "experimental".

This command is experimental and may be modified or removed in the future. Use
with caution in production.
`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Aliases:      []string{"graphql"},
		RunE: func(_ *cobra.Command, args []string) error {
			return g.Run(args[0])
		},
	}

	cmd.Flags().StringVar(
		&g.httpAddr,
		"server.graphql.endpoint",
		g.httpAddr,
		"Address of the GraphQL endpoint",
	)

	return cmd
}

func (g *alloyGql) Run(query string) error {
	c := client.NewGraphQLClient(g.httpAddr)

	response, err := c.Execute(formatGraphQLQuery(query))
	if err != nil {
		return fmt.Errorf("execute GraphQL query: %w", err)
	}

	utils.PrintGraphQLResponse(response)
	return nil
}

func formatGraphQLQuery(query string) string {
	trimmedQuery := strings.TrimSpace(query)

	for _, prefix := range []string{"{", "query", "mutation", "subscription"} {
		if strings.HasPrefix(trimmedQuery, prefix) {
			return query
		}
	}

	return "query { " + query + " }"
}
