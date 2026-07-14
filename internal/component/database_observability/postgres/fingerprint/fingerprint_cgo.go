//go:build cgo && !windows

package fingerprint

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var sentinelUnparsableFp = FingerprintOf(SentinelUnparsable)

// Supported reports whether SQL fingerprinting is available in this build.
// libpg_query requires cgo.
func Supported() bool { return true }

// Fingerprint parses query and returns its fingerprint, falling back to a
// quote/paren repair pass and then to the unparsable sentinel hash.
func Fingerprint(query string) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "", ErrEmpty
	}

	if fp, perr := pg_query.Fingerprint(query); perr == nil {
		return fp, nil
	}

	if fp, perr := pg_query.Fingerprint(repair(query)); perr == nil {
		return fp, nil
	}

	return sentinelUnparsableFp, nil
}

// FingerprintOf hashes a known sentinel string deterministically.
func FingerprintOf(text string) string {
	if fp, err := pg_query.Fingerprint(text); err == nil && fp != "" {
		return fp
	}
	return fmt.Sprintf("%016x", pg_query.HashXXH3_64([]byte(text), 0xee))
}

// repair closes unclosed single/double quotes and balances unclosed parens.
// Quote-balancing runs first: a string ending in `'(` needs the quote closed
// before the paren count is meaningful. Doubled-apostrophe escapes,
// dollar-quoted strings, and backslash-escaped quotes are miscounted.
func repair(query string) string {
	if strings.Count(query, "'")%2 == 1 {
		query += "'"
	}
	if strings.Count(query, "\"")%2 == 1 {
		query += "\""
	}
	if open := strings.Count(query, "(") - strings.Count(query, ")"); open > 0 {
		query += strings.Repeat(")", open)
	}
	return query
}
