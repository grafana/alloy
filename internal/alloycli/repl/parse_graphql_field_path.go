package repl

import (
	"strings"
)

// parseGraphQLFieldPath parses a GraphQL query fragment and returns the field path
// representing the parent context where the cursor is located.
func GetParentFieldPath(textBeforeCursor string) []string {
	text := strings.TrimSpace(textBeforeCursor)

	if text == "" {
		return []string{}
	}

	// Remove parentheses groups (field arguments)
	text = removeParenGroups(text)

	// Replace commas with spaces for consistent parsing
	text = strings.ReplaceAll(text, ",", " ")

	var fieldPath []string
	var currentField strings.Builder

	for _, char := range text {
		switch char {
		case '{':
			// Opening brace - the current field opens a new level
			if currentField.Len() > 0 {
				fieldPath = append(fieldPath, currentField.String())
				currentField.Reset()
			}
		case '}':
			// Closing brace - we're exiting a level
			if len(fieldPath) > 0 {
				fieldPath = fieldPath[:len(fieldPath)-1]
			}
			currentField.Reset()
		case ' ':
			// Space - new field at current level, reset current field
			currentField.Reset()
		default:
			// Regular character - building field name
			currentField.WriteRune(char)
		}
	}

	return fieldPath
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
