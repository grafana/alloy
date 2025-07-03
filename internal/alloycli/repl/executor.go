package repl

import (
	"fmt"
	"os"
	"strings"

	"github.com/grafana/alloy/internal/service/graphql"
)

type executor struct {
	cfg       *AlloyRepl
	gqlClient *graphql.GraphQlClient
}

func NewExecutor(cfg *AlloyRepl, gqlClient *graphql.GraphQlClient) *executor {
	return &executor{
		cfg:       cfg,
		gqlClient: gqlClient,
	}
}

func (e *executor) Execute(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if line == "exit" || line == "quit" {
		fmt.Println("Exiting Alloy REPL.")
		os.Exit(0)
	}

	// Wrap the query in query {...} for convenience
	if !strings.HasPrefix(line, "query") {
		line = "query { " + line + " }"
	}

	response, err := e.gqlClient.Execute(line)
	if err != nil {
		fmt.Printf("Error executing query: %v\n", err)
		return
	}

	printGraphQlResponse(response)
}
