package sqlfingerprint

import "strings"

// normalizePostgresObfuscatedComment treats the final standalone question mark
// as a comment erased by Ruby OpenTelemetry, regardless of the preceding clause.
// Remaining question marks are values only where an operand is expected; infix
// JSON operators survive. The caller still validates the resulting statement.
// Existing successful v1 inputs are unaffected because this suffix was rejected.
func normalizePostgresObfuscatedComment(tokens []token) []token {
	if len(tokens) == 0 || tokens[len(tokens)-1] != (token{symbol, "?"}) {
		return tokens
	}
	tokens = tokens[:len(tokens)-1]
	for i, t := range tokens {
		if t == (token{symbol, "?"}) && postgresObfuscatedValueExpected(tokens[:i]) {
			tokens[i] = token{value, "?"}
		}
	}
	return tokens
}

// postgresObfuscatedValueExpected reuses expression context before keyword case
// normalization. It never folds quoted identifiers or mutates preceding tokens.
func postgresObfuscatedValueExpected(previous []token) bool {
	if len(previous) == 0 {
		return false
	}
	last := previous[len(previous)-1]
	if last.kind == word {
		last.text = strings.ToLower(last.text)
	}
	return expectsValue([]token{last})
}
