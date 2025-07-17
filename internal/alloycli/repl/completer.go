package repl

import (
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/grafana/alloy/internal/service/graphql"
)

type completer struct {
	cfg       *AlloyRepl
	gqlClient *graphql.GraphQlClient
	lastError string
}

var topLevelCommands = []prompt.Suggest{
	{
		Text:        "exit",
		Description: "Exit the REPL",
	},
	{
		Text:        "quit",
		Description: "Quit the REPL",
	},
	{
		Text:        "help",
		Description: "Show help",
	},
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

	nearestBracket := findPreviousBracket(d.TextBeforeCursor())
	parentPath := GetParentFieldPath(d.TextBeforeCursor())

	suggestions := []prompt.Suggest{}

	switch nearestBracket {
	case '(':
		suggestions = c.suggestArguments(parentPath)

	case '{', '}', ')', 0:
		suggestions = c.suggestFields(parentPath)
	}

	if len(parentPath) == 0 {
		suggestions = append(suggestions, topLevelCommands...)
	}

	curWord := GetGraphQlWordBeforeCursor(d)

	return prompt.FilterHasPrefix(suggestions, curWord, true)
}

// GetGraphQlWordBeforeCursor gets the current word before the cursor, accounting for word
// boundaries in graphql which include "{" and "("
func GetGraphQlWordBeforeCursor(doc prompt.Document) string {
	word := doc.GetWordBeforeCursor()
	word = strings.ReplaceAll(word, "{", " ")
	word = strings.ReplaceAll(word, "(", " ")

	if strings.HasSuffix(word, " ") {
		word = ""
	} else {
		parts := strings.Fields(word)
		if len(parts) > 0 {
			word = parts[len(parts)-1]
		}
	}

	return word
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

// findPreviousBracket finds the nearest bracket (paren or curly brace) to the left of the current
// cursor position
func findPreviousBracket(textBeforeCursor string) rune {
	for i := len(textBeforeCursor) - 1; i >= 0; i-- {
		char := rune(textBeforeCursor[i])
		if brackets[char] {
			return char
		}
	}

	return 0
}

func (c *completer) suggestArguments(_ []string) []prompt.Suggest {
	// TODO: autocomplete argument names
	return []prompt.Suggest{}
}

func (c *completer) suggestFields(parentPath []string) []prompt.Suggest {
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

	return fieldSuggestions
}
