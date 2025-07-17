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

	text = removeParenGroups(text)

	// Allow for easier tokenization with strategic substitutions
	text = strings.ReplaceAll(text, ",", " ")
	text = strings.ReplaceAll(text, "{", " { ")
	text = strings.ReplaceAll(text, "}", " } ")

	tokens := strings.FieldsSeq(text)

	var path []string
	var prev string
	for token := range tokens {
		switch token {
		case "{":
			// Start new level. Append prev field to path
			path = append(path, prev)
		case "}":
			// End current level. Remove prev field from path
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
		default:
			// Non-paren token (aka a field). Do nothing
		}
		prev = token
	}

	return path
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
