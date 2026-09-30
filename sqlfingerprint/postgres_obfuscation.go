package sqlfingerprint

import "strings"

// stripPostgresObfuscatedComment treats the final standalone question mark as a
// comment erased by Ruby OpenTelemetry, regardless of the preceding clause.
// The caller still validates the remaining statement after normalizing values.
func stripPostgresObfuscatedComment(tokens []token) []token {
	if len(tokens) > 0 && tokens[len(tokens)-1] == (token{symbol, "?"}) {
		return tokens[:len(tokens)-1]
	}
	return tokens
}

// normalizePostgresParameters recognizes JDBC and obfuscated value placeholders
// wherever an operand is expected, whether or not a comment marker was present.
// Infix JSON operators and question marks inside quoted tokens are preserved.
func normalizePostgresParameters(tokens []token) []token {
	for i, t := range tokens {
		if t == (token{symbol, "?"}) && postgresValueExpected(tokens[:i]) {
			tokens[i] = token{value, "?"}
		}
	}
	return tokens
}

// postgresValueExpected reuses expression context before keyword case
// normalization. It never folds quoted identifiers or mutates preceding tokens.
func postgresValueExpected(previous []token) bool {
	if len(previous) == 0 {
		return false
	}
	last := previous[len(previous)-1]
	if last.kind == word {
		last.text = strings.ToLower(last.text)
		if last.text == "ilike" {
			return true
		}
	}
	return expectsValue([]token{last})
}
