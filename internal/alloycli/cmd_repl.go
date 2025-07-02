package alloycli

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/alloycli/repl"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/graphql"
)

type alloyRepl struct {
	httpAddr string
	// storagePath          string
	minStability featuregate.Stability
	// uiPrefix     string
	// configFormat         string
	enableCommunityComps bool
}

type executor struct {
	cfg       *alloyRepl
	gqlClient *graphql.GraphQlClient
}

type completer struct {
	cfg       *alloyRepl
	gqlClient *graphql.GraphQlClient
	lastError string
}

var controlChars = map[rune]bool{
	'"': true,
	'(': true,
	')': true,
	'{': true,
	'}': true,
}

func replCommand() *cobra.Command {
	r := &alloyRepl{
		httpAddr: "http://127.0.0.1:12345/graphql",
		// storagePath:    "data-alloy/",
		minStability: featuregate.StabilityGenerallyAvailable,
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
			&r.httpAddr,
			"server.graphql.endpoint",
			r.httpAddr,
			"Address of the GraphQL endpoint",
		)
	// cmd.Flags().StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to discover the HTTP UI at")

	// Config flags
	// cmd.Flags().StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	// cmd.Flags().BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")
	// cmd.Flags().StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	// cmd.Flags().StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	cmd.Flags().Var(&r.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	// cmd.Flags().BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")

	return cmd
}

func (fr *alloyRepl) Run(cmd *cobra.Command) error {
	client := graphql.NewGraphQlClient(fr.httpAddr)

	p := prompt.New(
		NewExecutor(fr, client).Execute,
		NewCompleter(fr, client).Complete,
		prompt.OptionTitle("alloy-repl: interactive alloy diagnostics"),
		prompt.OptionPrefix("alloy >> "),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{'('},
			Fn:        insertCharPair("(  )"),
		}),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{'{'},
			Fn:        insertCharPair("{  }"),
		}),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{'"'},
			Fn:        insertCharPair("\"\""),
		}),
	)
	p.Run()

	return nil
}

func insertCharPair(pair string) func(buf *prompt.Buffer) {
	return func(buf *prompt.Buffer) {
		buf.InsertText(pair, false, false)
		buf.CursorRight(len(pair) / 2)
	}
}

func NewExecutor(cfg *alloyRepl, gqlClient *graphql.GraphQlClient) *executor {
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

	repl.PrintGraphQlResponse(response)
}

func NewCompleter(cfg *alloyRepl, gqlClient *graphql.GraphQlClient) *completer {
	return &completer{cfg: cfg, gqlClient: gqlClient}
}

func (c *completer) Complete(d prompt.Document) []prompt.Suggest {
	// Find the nearest control character to the right of cursor
	nearestControlChar := findNearestControlChar(d)

	// If nearest control character is a quote, don't suggest anything
	if nearestControlChar == '"' {
		return []prompt.Suggest{}
	}

	// Determine if the cursor is inside a parentheses pair
	if nearestControlChar == '(' {
		return []prompt.Suggest{
			{
				Text: "we are in a parentheses!",
			},
		}
	}

	// Determine if the cursor is inside a curly brace pair
	if nearestControlChar == '{' {
		suggestions := []prompt.Suggest{
			// {
			// 	Text: strings.Join(parseGraphQLFieldPath(d.TextBeforeCursor()), "->"),
			// },
			{Text: "suggestion1"},
			{Text: "suggestion2"},
			{Text: "suggestion3"},
		}
		return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
	}

	// Assume we are not inside a bracket of some sort
	response, err := repl.IntrospectQueryFields(c.gqlClient)
	if err != nil {
		errorMsg := fmt.Sprintf("%v", err)

		// Only display error if it's different from the last one
		if errorMsg != c.lastError {
			fmt.Println("Error introspecting schema. Is Alloy running?")
			fmt.Println(errorMsg)
			c.lastError = errorMsg
		}
		return []prompt.Suggest{}
	}

	// Reset error state on successful introspection
	c.lastError = ""

	fields := make([]prompt.Suggest, len(response))
	for i, field := range response {
		fields[i] = prompt.Suggest{
			Text:        field.Name,
			Description: field.Description,
		}
	}

	return prompt.FilterHasPrefix(fields, d.GetWordBeforeCursor(), true)
}

// parseGraphQLFieldPath parses a GraphQL query fragment and returns the field path
// representing the current cursor context. For example:
// - "components{" -> ["components"]
// - "components{name, config{" -> ["components", "config"]
// - "components(id: 123){config{" -> ["components", "config"]
func parseGraphQLFieldPath(textBeforeCursor string) []string {
	// Remove query wrapper if present
	text := strings.TrimSpace(textBeforeCursor)

	if text == "" {
		return []string{}
	}

	// Remove all whitespace characters
	text = removeWhitespace(text)

	// Remove parentheses groups (field arguments)
	text = removeParenGroups(text)

	// Split on opening curly braces
	parts := strings.Split(text, "{")

	var fieldPath []string
	for i, part := range parts {
		// Skip the last empty part if it exists
		if i == len(parts)-1 && part == "" {
			continue
		}

		// Extract the field name (last field after comma)
		fields := strings.Split(part, ",")
		if len(fields) > 0 {
			fieldName := fields[len(fields)-1]
			if fieldName != "" {
				fieldPath = append(fieldPath, fieldName)
			}
		}
	}

	return fieldPath
}

// removeWhitespace removes all whitespace characters from the text
func removeWhitespace(text string) string {
	var result strings.Builder
	for _, char := range text {
		if char != ' ' && char != '\t' && char != '\n' && char != '\r' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

// removeParenGroups removes parentheses groups like (arg: value) from the text
func removeParenGroups(text string) string {
	var result strings.Builder
	parenDepth := 0

	for _, char := range text {
		if char == '(' {
			parenDepth++
		} else if char == ')' {
			parenDepth--
		} else if parenDepth == 0 {
			result.WriteRune(char)
		}
	}

	return result.String()
}

// findNearestControlChar finds the nearest control character (quote, curly brace, or paren)
// to the left of the current cursor position
func findNearestControlChar(d prompt.Document) rune {
	textBeforeCursor := d.TextBeforeCursor()

	for i := len(textBeforeCursor) - 1; i >= 0; i-- {
		char := rune(textBeforeCursor[i])
		if controlChars[char] {
			return char
		}
	}

	// Return null rune if no control character found
	return 0
}
