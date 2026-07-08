//go:build cgo

// Package fingerprint computes stable, semantic SQL fingerprints via
// libpg_query. The fingerprint is identical across comment/whitespace
// differences and literal-vs-placeholder differences. libpg_query is cgo-only;
// the !cgo build is provided by fingerprint_nocgo.go and returns ErrEmpty
// from every call.
package fingerprint

import (
	"errors"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Source records where a query text came from. Both current sources carry
// untruncated text, so an unparsable query always maps to the unparsable
// sentinel; the value is retained so callers can record query provenance.
type Source int

const (
	SourcePgStatStatements Source = iota
	SourceLog
)

const SentinelUnparsable = "<unparsable query>"

var ErrEmpty = errors.New("fingerprint: empty query text")

var sentinelUnparsableFp = FingerprintOf(SentinelUnparsable)

// Fingerprint parses query and returns its fingerprint, falling back to a
// quote/paren repair pass and then to the unparsable sentinel hash.
func Fingerprint(query string, source Source) (fp string, repaired bool, err error) {
	if strings.TrimSpace(query) == "" {
		return "", false, ErrEmpty
	}

	if fp, perr := pg_query.Fingerprint(query); perr == nil {
		return fp, false, nil
	}

	if fp, perr := pg_query.Fingerprint(repair(query)); perr == nil {
		return fp, true, nil
	}

	return sentinelUnparsableFp, true, nil
}

// FingerprintOf hashes a known sentinel string deterministically.
func FingerprintOf(text string) string {
	if fp, err := pg_query.Fingerprint(text); err == nil && fp != "" {
		return fp
	}
	return formatHash(pg_query.HashXXH3_64([]byte(text), 0xee))
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
	open := strings.Count(query, "(") - strings.Count(query, ")")
	for i := 0; i < open; i++ {
		query += ")"
	}
	return query
}

func formatHash(h uint64) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		out[i] = hexChars[h&0xF]
		h >>= 4
	}
	return string(out)
}
