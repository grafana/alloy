package repl

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/grafana/alloy/internal/service/graphql"
)

type completer struct {
	cfg       *AlloyRepl
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

func NewCompleter(cfg *AlloyRepl, gqlClient *graphql.GraphQlClient) *completer {
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

	parentPath := GetParentFieldPath(d.TextBeforeCursor())

	// fmt.Printf("parentPath: %v\n", parentPath)

	response, err := Introspect(c.gqlClient)
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

	fields := response.GetFieldsAtPath(parentPath)

	fieldSuggestions := make([]prompt.Suggest, len(fields))
	for i, field := range fields {
		if field.Description != nil {
			fieldSuggestions[i] = prompt.Suggest{
				Text:        field.Name,
				Description: *field.Description,
			}
		} else {
			fieldSuggestions[i] = prompt.Suggest{
				Text: field.Name,
			}
		}
	}

	return prompt.FilterHasPrefix(fieldSuggestions, d.GetWordBeforeCursor(), true)
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
