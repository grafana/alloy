package utils

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/grafana/alloy/internal/service/graphql/client"
)

// PrintGraphQLResponse pretty prints a GraphQL response.
func PrintGraphQLResponse(writer io.Writer, parsedResponse *client.GraphQLResponse) error {
	prettyBytes, err := json.MarshalIndent(parsedResponse, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GraphQL response: %w", err)
	}
	if _, err := fmt.Fprintln(writer, string(prettyBytes)); err != nil {
		return fmt.Errorf("write GraphQL response: %w", err)
	}
	return nil
}
