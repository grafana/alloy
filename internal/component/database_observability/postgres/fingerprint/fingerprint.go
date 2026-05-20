// Package fingerprint computes stable, semantic SQL fingerprints for PostgreSQL
// query text using the libpg_query parser (via pg_query_go).
//
// The same fingerprint is produced regardless of comments, whitespace, or
// literal-vs-placeholder differences, allowing pg_stat_statements metrics,
// pg_stat_activity samples, and server-log query text to be correlated by a
// single client-side identifier — including on managed services (RDS, Aurora)
// that do not expose pg_stat_statements.queryid in log_line_prefix.
package fingerprint

import (
	"errors"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Source describes which PostgreSQL surface the query text was read from.
// It only affects the sentinel chosen when both parse and repair fail.
type Source int

const (
	SourcePgStatStatements Source = iota
	SourcePgStatActivity
	SourceLog
)

// Sentinel strings used when query text cannot be parsed even after repair.
// These match the sentinels used by pganalyze's collector so existing
// dashboards built around their values port over without changes.
const (
	SentinelTruncated  = "<truncated query>"
	SentinelUnparsable = "<unparsable query>"
)

// ErrEmpty is returned when the input is empty or whitespace-only. Callers
// should skip emitting fingerprints for these (don't even use a sentinel).
var ErrEmpty = errors.New("fingerprint: empty query text")

var (
	sentinelTruncatedFp  = FingerprintOf(SentinelTruncated)
	sentinelUnparsableFp = FingerprintOf(SentinelUnparsable)
)

// Fingerprint parses the query and returns its fingerprint, falling back to a
// quote/paren repair pass and then to a sentinel hash. trackActivityQuerySize
// is only consulted when source == SourcePgStatActivity. The repaired flag is
// true whenever the input didn't parse as-is; compare the returned value
// against FingerprintOf(Sentinel*) to distinguish a successful repair from a
// sentinel fallback.
func Fingerprint(query string, source Source, trackActivityQuerySize int) (fp string, repaired bool, err error) {
	if strings.TrimSpace(query) == "" {
		return "", false, ErrEmpty
	}

	if fp, perr := pg_query.Fingerprint(query); perr == nil {
		return fp, false, nil
	}

	fixed := repair(query)
	if fp, perr := pg_query.Fingerprint(fixed); perr == nil {
		return fp, true, nil
	}

	return sentinelFingerprint(query, source, trackActivityQuerySize), true, nil
}

// FingerprintOf hashes a known sentinel string deterministically. Exported so
// tests and callers can compare against the values produced for sentinels.
func FingerprintOf(text string) string {
	if fp, err := pg_query.Fingerprint(text); err == nil && fp != "" {
		return fp
	}
	// pg_query.Fingerprint may not parse a non-SQL sentinel; fall back to a
	// deterministic hash of the text (matches pganalyze's util/fingerprint.go).
	return formatHash(pg_query.HashXXH3_64([]byte(text), 0xee))
}

func sentinelFingerprint(query string, source Source, trackActivityQuerySize int) string {
	if source == SourcePgStatActivity && trackActivityQuerySize > 0 && len(query) == trackActivityQuerySize-1 {
		return sentinelTruncatedFp
	}
	return sentinelUnparsableFp
}

// repair closes unclosed single/double quotes and balances unclosed
// parentheses, mirroring pganalyze's `fixTruncatedQuery`. Known false
// positives: doubled-apostrophe escapes (`'O''Brien'`), dollar-quoted strings
// (`$body$ ... $body$`), and backslash-escaped quotes with
// `standard_conforming_strings = off`. Quote-balancing must run before
// paren-balancing — a string ending in `'(` should have the quote closed first.
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
