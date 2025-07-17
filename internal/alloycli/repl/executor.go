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
	commands  map[string]func()
}

func NewExecutor(cfg *AlloyRepl, gqlClient *graphql.GraphQlClient) *executor {
	e := &executor{
		cfg:       cfg,
		gqlClient: gqlClient,
	}

	// Initialize command map
	e.commands = map[string]func(){
		"exit": e.exitCommand,
		"quit": e.exitCommand,
		"help": e.helpCommand,
	}

	return e
}

func (e *executor) Execute(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Check for built-in commands first
	if handler, exists := e.commands[line]; exists {
		handler()
		return
	}

	// Wrap in query {...} for convenience. If we ever add mutations, this will need to be updated.
	if !strings.HasPrefix(line, "query") {
		line = "query { " + line + " }"
	}

	e.executeQuery(line)
}

func (e *executor) executeQuery(query string) {
	response, err := e.gqlClient.Execute(query)
	if err != nil {
		fmt.Println("Error executing query. Is Alloy running?")
		fmt.Printf("%v\n", err)
		return
	}

	printGraphQlResponse(response)
}

func (e *executor) exitCommand() {
	fmt.Println("Exiting Alloy REPL.")
	os.Exit(0)
}

func (e *executor) helpCommand() {
	fmt.Print(
		`
Welcome to the Alloy REPL! This is a diagnostic tool that interacts with a
GraphQL server running in Alloy. It exposes several useful data points from
within Alloy and can assist with understanding if networking is properly
established between Alloy and its targets.

Queries are entered in single-line GraphQL syntax, for example:

  alloy >> components{ id, name, health{ status } }

To learn more about GraphQL, see https://graphql.org/learn/

Available commands:
	exit, quit - Exit the REPL
	help       - Show this help message
	<query>    - Execute a GraphQL query
`,
	)
}
