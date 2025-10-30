package database_observability

import (
	"fmt"
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

// ContainsReservedKeywords checks if the SQL query contains any reserved keywords
// that indicate write operations, excluding those in string literals or comments
func ContainsReservedKeywords(query string, reservedWords []string) bool {
	// Create a map for faster lookup
	reservedMap := make(map[string]bool)
	for _, word := range reservedWords {
		reservedMap[strings.ToUpper(word)] = true
	}

	// Use the lexer to tokenize the query
	lexer := sqllexer.New(query, sqllexer.WithDBMS(sqllexer.DBMSPostgres))

	// Scan all tokens
	for {
		token := lexer.Scan()
		if token.Type == sqllexer.EOF {
			break
		}
		if token.Type == sqllexer.ERROR {
			// If lexing fails, fall back to simple string search for safety
			uppercaseQuery := strings.ToUpper(query)
			for _, word := range reservedWords {
				if strings.Contains(uppercaseQuery, word) {
					return true
				}
			}
			return false
		}

		// Check commands, keywords, and identifiers (since some reserved words might be classified as identifiers)
		// but exclude string literals, comments, and other non-SQL-keyword tokens
		if token.Type == sqllexer.COMMAND || token.Type == sqllexer.KEYWORD || token.Type == sqllexer.IDENT {
			if reservedMap[strings.ToUpper(token.Value)] {
				return true
			}
		}
	}

	return false
}
