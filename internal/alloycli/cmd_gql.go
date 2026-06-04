package alloycli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/service/graphql/client"
	"github.com/grafana/alloy/internal/service/graphql/utils"
)

type alloyGql struct {
	endpoint string
}

var operationRegex = regexp.MustCompile(`^((query|mutation|subscription)(\s|\{)|(\{))`)

func gqlCommand() *cobra.Command {
	g := &alloyGql{
		endpoint: "http://127.0.0.1:12345/graphql",
	}

	cmd := &cobra.Command{
		Use:   "gql <query>",
		Short: "[EXPERIMENTAL] Run a GraphQL query against a running Alloy instance",
		Long: `The gql subcommand runs a GraphQL query against a running Alloy instance.
The query is provided as a single argument to the command.

It requires the --feature.graphql.enabled flag on the running Alloy instance to
be set, as well as --stability.level flag set to "experimental".

This command is experimental and may be modified or removed in the future. Use
with caution in production.
`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Aliases:      []string{"graphql"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.Run(args[0], cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(
		&g.endpoint,
		"endpoint",
		g.endpoint,
		"Address of the GraphQL endpoint",
	)

	return cmd
}

func (g *alloyGql) Run(query string, out io.Writer) error {
	c := client.NewGraphQLClient(g.endpoint)

	formattedQuery, err := formatGraphQLQuery(query)
	if err != nil {
		return fmt.Errorf("format GraphQL query: %w", err)
	}

	response, err := c.Execute(formattedQuery)
	if err != nil {
		return fmt.Errorf("execute GraphQL query: %w", err)
	}

	if err := utils.PrintGraphQLResponse(out, response); err != nil {
		return fmt.Errorf("print GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL response contains errors")
	}
	return nil
}

func formatGraphQLQuery(query string) (string, error) {
	trimmedQuery := strings.TrimSpace(query)

	firstParen := strings.Index(trimmedQuery, "(")
	firstCurly := strings.Index(trimmedQuery, "{")
	if firstParen >= 0 && (firstCurly == -1 || firstParen < firstCurly) {
		return "", fmt.Errorf("query parameters are not supported")
	}

	if operationRegex.MatchString(trimmedQuery) {
		return query, nil
	}

	return "query { " + query + " }", nil
}
