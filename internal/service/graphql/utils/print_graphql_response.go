package utils

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/alloy/internal/service/graphql/client"
)

func PrintGraphQlResponse(parsedResponse *client.GraphQlResponse) {
	prettyBytes, err := json.MarshalIndent(parsedResponse.Data, "", "  ")
	if err != nil {
		fmt.Printf("Failed to pretty print response data: %v\n", err)
		return
	}
	fmt.Println(string(prettyBytes))

	if len(parsedResponse.Errors) > 0 {
		prettyBytes, err = json.MarshalIndent(parsedResponse.Errors, "", "  ")
		if err != nil {
			fmt.Printf("Failed to pretty print response errors: %v\n", err)
			return
		}
		fmt.Println("Errors: ", string(prettyBytes))
	}
}
