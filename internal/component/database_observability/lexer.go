package database_observability

import (
	"fmt"
	"strings"

	"github.com/DataDog/go-sqllexer"
)

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
