package database_observability

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/DataDog/go-sqllexer"
)

// ExtractTableNames extracts the table names from a SQL query
func ExtractTableNames(sql string) ([]string, error) {
	normalizer := sqllexer.NewNormalizer(
		sqllexer.WithCollectTables(true),
	)
	_, metadata, err := normalizer.Normalize(sql, sqllexer.WithDBMS(sqllexer.DBMSPostgres))
	if err != nil {
		return nil, fmt.Errorf("failed to normalize SQL: %w", err)
	}

	// Return all table names, including those that end with "..." for truncated queries, as we can't know if the table name was truncated or not
	return metadata.Tables, nil
}

// RedactSql obfuscates a SQL query by replacing literals with ? placeholders
func RedactSql(sql string) string {
	obfuscatedSql := sqllexer.NewObfuscator().Obfuscate(sql)
	return obfuscatedSql
}

func isCommandKeywordOrIdentifier(token sqllexer.Token) bool {
	return token.Type == sqllexer.COMMAND || token.Type == sqllexer.KEYWORD || token.Type == sqllexer.IDENT
}

// ContainsReservedKeywords checks if the SQL query contains any reserved keywords
// that indicate write operations, excluding those in string literals or comments
func ContainsReservedKeywords(query string, reservedWords map[string]ExplainReservedWordMetadata, dbms sqllexer.DBMSType) (bool, error) {
	// Use the lexer to tokenize the query
	lexer := sqllexer.New(query, sqllexer.WithDBMS(dbms))
	tokenBuffer := make([]sqllexer.Token, 0)

	// Scan all tokens
	for {
		token := lexer.Scan()
		if token.Type == sqllexer.ERROR {
			return false, fmt.Errorf("lexer failed to scan query, offending token value: %s", token.Value)
		}
		if token.Type == sqllexer.EOF {
			break
		}
		tokenBuffer = append(tokenBuffer, *token)
	}

	for tIdx, token := range tokenBuffer {
		if isCommandKeywordOrIdentifier(token) {
			if resWord, ok := reservedWords[strings.ToUpper(token.Value)]; ok {
				if resWord.ExemptionPrefixes != nil && len(*resWord.ExemptionPrefixes) > 0 {
					lookbackCount := len(*resWord.ExemptionPrefixes)
					skippedTokens := 0
					matchedTokens := 0
					currentExemptionPrefix := (*resWord.ExemptionPrefixes)[0]
					for i := tIdx - 1; i >= 0; i-- {
						if tokenBuffer[i].Type == sqllexer.SPACE {
							skippedTokens++
							continue
						}
						if isCommandKeywordOrIdentifier(tokenBuffer[i]) {
							if strings.EqualFold(tokenBuffer[i].Value, currentExemptionPrefix) {
								matchedTokens++
								if matchedTokens < lookbackCount {
									currentExemptionPrefix = (*resWord.ExemptionPrefixes)[matchedTokens]
								}
							}
						}
					}
					if matchedTokens < lookbackCount {
						return true, nil
					}
				} else {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// RedactParenthesizedValues redacts literal values within parentheses to protect PII.
// For patterns like "(column)=(value)", only the value part is redacted: "(column)=(?)".
// For other patterns like "Failing row contains (data)", the content is redacted: "Failing row contains (?)".
func RedactParenthesizedValues(text string) string {
	if text == "" {
		return text
	}

	const placeholder = "\x00PROTECTED_PARENS\x00"

	equalsPattern := regexp.MustCompile(`\(([^)]*)\)=`)
	protectedParts := []string{}
	text = equalsPattern.ReplaceAllStringFunc(text, func(match string) string {
		protectedParts = append(protectedParts, match)
		return placeholder
	})

	equalsValuePattern := regexp.MustCompile(`\x00PROTECTED_PARENS\x00\(([^)]*)\)`)
	text = equalsValuePattern.ReplaceAllString(text, placeholder+"(?)")

	standalonePattern := regexp.MustCompile(`\([^)]*\)`)
	text = standalonePattern.ReplaceAllString(text, "(?)")

	for _, protected := range protectedParts {
		text = strings.Replace(text, placeholder, protected, 1)
	}

	return text
}

// RedactSQLWithinMixedText finds and redacts SQL statements within mixed text that could contain PII.
func RedactSQLWithinMixedText(text string) string {
	if text == "" {
		return text
	}

	sqlKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "MERGE",
		"WITH", "COPY", "DO", "CALL", "EXECUTE", "PREPARE",
		"CREATE USER", "CREATE ROLE", "ALTER USER", "ALTER ROLE", "DROP USER", "DROP ROLE",
		"GRANT", "REVOKE", "SET", "VALUES",
	}

	result := text

	for _, keyword := range sqlKeywords {
		escapedKeyword := strings.ReplaceAll(keyword, " ", `\s+`)
		pattern := fmt.Sprintf(`(?i)\b%s\b[^;]*(?:;|$)`, escapedKeyword)
		re := regexp.MustCompile(pattern)

		matches := re.FindAllString(result, -1)
		for _, match := range matches {
			redacted := RedactSql(match)
			result = strings.Replace(result, match, redacted, 1)
		}
	}

	return result
}
