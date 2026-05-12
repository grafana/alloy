package utils

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/grafana/alloy/internal/service/graphql/client"
)

// PrintGraphQLResponse pretty prints a GraphQL response.
func PrintGraphQLResponse(writer io.Writer, parsedResponse *client.GraphQLResponse) error {
	prettyBytes, err := json.MarshalIndent(parsedResponse.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GraphQL response data: %w", err)
	}
	if _, err := fmt.Fprintln(writer, string(prettyBytes)); err != nil {
		return fmt.Errorf("write GraphQL response data: %w", err)
	}

	if len(parsedResponse.Errors) > 0 {
		prettyBytes, err = json.MarshalIndent(parsedResponse.Errors, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal GraphQL response errors: %w", err)
		}
		if _, err := fmt.Fprintln(writer, "Errors: ", string(prettyBytes)); err != nil {
			return fmt.Errorf("write GraphQL response errors: %w", err)
		}
	}
	return nil
}
