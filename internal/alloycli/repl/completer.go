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

var brackets = map[rune]bool{
	'(': true,
	')': true,
	'{': true,
	'}': true,
}

func NewCompleter(cfg *AlloyRepl, gqlClient *graphql.GraphQlClient) *completer {
	return &completer{cfg: cfg, gqlClient: gqlClient}
}

func (c *completer) Complete(d prompt.Document) []prompt.Suggest {
	// First check if we're inside a quoted string
	if isInsideQuotedString(d.TextBeforeCursor()) {
		return []prompt.Suggest{}
	}

	// Find the nearest bracket to the left of cursor
	nearestBracket := findNearestBracket(d)

	// Determine if the cursor is inside a parentheses pair
	if nearestBracket == '(' {
		// TODO: autocomplete argument names
		return []prompt.Suggest{}
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

// isInsideQuotedString determines if the cursor is currently inside a quoted string
// accounting for escaped quotes (\")
func isInsideQuotedString(text string) bool {
	insideQuote := false
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		// If we encounter a backslash, skip the next character (it's escaped)
		if char == '\\' && i+1 < len(runes) {
			i++ // Skip the escaped character
			continue
		}

		// If we encounter an unescaped quote, toggle the quote state
		if char == '"' {
			insideQuote = !insideQuote
		}
	}

	return insideQuote
}

// findNearestBracket finds the nearest bracket (paren or curly brace)
// to the left of the current cursor position
func findNearestBracket(d prompt.Document) rune {
	textBeforeCursor := d.TextBeforeCursor()

	for i := len(textBeforeCursor) - 1; i >= 0; i-- {
		char := rune(textBeforeCursor[i])
		if brackets[char] {
			return char
		}
	}

	// Return null rune if no bracket found
	return 0
}
