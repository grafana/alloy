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

// bufSize bounds the number of recent command/keyword/identifier (CKI) tokens
// stored for exemption-prefix matching. It is fixed so the ring lives on
// the stack instead of the heap. The size is large enough to accommodate the
// longest ExemptionPrefixes list.
const bufSize = 32

// ContainsReservedKeywords checks if the SQL query contains any reserved keywords
// that indicate write operations, excluding those in string literals or comments
func ContainsReservedKeywords(query string, reservedWords map[string]ExplainReservedWordMetadata, dbms sqllexer.DBMSType) (bool, error) {
	lexer := sqllexer.New(query, sqllexer.WithDBMS(dbms))

	// buf is a ringbuffer that stores the most recent tokens.
	// bufHead points at the next write slot; once bufLen
	// reaches bufSize the oldest entry is overwritten.
	var buf [bufSize]string
	bufHead := 0
	bufLen := 0

	for {
		token := lexer.Scan()
		switch token.Type {
		case sqllexer.ERROR:
			return false, fmt.Errorf("lexer failed to scan query, offending token value: %s", token.Value)
		case sqllexer.EOF:
			return false, nil
		}

		if !isCommandKeywordOrIdentifier(*token) {
			continue
		}

		if resWord, ok := reservedWords[strings.ToUpper(token.Value)]; ok {
			if resWord.ExemptionPrefixes == nil || len(*resWord.ExemptionPrefixes) == 0 {
				return true, nil
			}

			// check for prefixes match in the ring buffer
			prefixes := *resWord.ExemptionPrefixes
			lookbackCount := len(prefixes)
			matchedTokens := 0
			currentExemptionPrefix := prefixes[0]

			for i := 0; i < bufLen; i++ {
				// walk the ringbuffer newest-to-oldest, the most recently
				// inserted entry is at bufHead-1 (mod bufSize)
				idx := (bufHead - 1 - i + bufSize) % bufSize
				if !strings.EqualFold(buf[idx], currentExemptionPrefix) {
					continue
				}
				matchedTokens++
				if matchedTokens >= lookbackCount {
					break
				}
				currentExemptionPrefix = prefixes[matchedTokens]
			}
			if matchedTokens < lookbackCount {
				return true, nil
			}
		}

		buf[bufHead] = token.Value
		bufHead = (bufHead + 1) % bufSize
		if bufLen < bufSize {
			bufLen++
		}
	}
}
