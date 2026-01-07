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
// For "(column)=(value)" patterns, only the value part is redacted: "(column)=(?)".
// Uses the SQL lexer's obfuscator to handle complex PostgreSQL syntax.
func RedactParenthesizedValues(text string) string {
	if text == "" {
		return text
	}

	result := strings.Builder{}
	result.Grow(len(text))
	obfuscator := sqllexer.NewObfuscator()
	i := 0

	for i < len(text) {
		if text[i] == '(' {
			depth := 1
			j := i + 1
			for j < len(text) && depth > 0 {
				switch text[j] {
				case '(':
					depth++
				case ')':
					depth--
				}
				j++
			}

			if depth == 0 && j < len(text) && text[j] == '=' && j+1 < len(text) && text[j+1] == '(' {
				result.WriteString(text[i:j])
				result.WriteByte('=')

				k := j + 2
				depth = 1
				for k < len(text) && depth > 0 {
					switch text[k] {
					case '(':
						depth++
					case ')':
						depth--
					}
					k++
				}

				if depth == 0 {
					valuePart := text[j+1 : k]
					obfuscated := obfuscator.Obfuscate(valuePart)
					result.WriteString(obfuscated)
					i = k
					continue
				}
			}

			if depth == 0 {
				result.WriteString("(?)")
				i = j
			} else {
				result.WriteByte(text[i])
				i++
			}
		} else {
			result.WriteByte(text[i])
			i++
		}
	}

	return result.String()
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
