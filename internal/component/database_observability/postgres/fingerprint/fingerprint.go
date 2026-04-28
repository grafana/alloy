// Package fingerprint computes stable, semantic SQL fingerprints for PostgreSQL
// query text using the libpg_query parser (via pg_query_go).
//
// The same fingerprint is produced by Alloy regardless of comments, whitespace,
// or literal-vs-placeholder differences, allowing pg_stat_statements metrics,
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

// Fingerprint runs the three-stage pipeline:
//  1. Parse the input as-is.
//  2. If parsing fails, balance unclosed quotes and parentheses and retry.
//  3. If parsing still fails, return a sentinel fingerprint.
//
// trackActivityQuerySize is the value of the postgres setting
// `track_activity_query_size` and is only consulted when source ==
// SourcePgStatActivity. Pass 0 for other sources.
//
// The returned `repaired` flag reports whether stage 2 was needed; callers may
// log this as a metric to detect upstream truncation issues.
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
	// deterministic hash of the text. pg_query.HashXXH3_64 is what pganalyze
	// uses for static texts (util/fingerprint.go).
	return formatHash(pg_query.HashXXH3_64([]byte(text), 0xee))
}

func sentinelFingerprint(query string, source Source, trackActivityQuerySize int) string {
	if source == SourcePgStatActivity && trackActivityQuerySize > 0 && len(query) == trackActivityQuerySize-1 {
		return FingerprintOf(SentinelTruncated)
	}
	return FingerprintOf(SentinelUnparsable)
}

// repair closes unclosed single/double quotes and balances unclosed
// parentheses, mirroring pganalyze's `fixTruncatedQuery`. The repaired text
// is only used for fingerprint computation — it is not emitted anywhere.
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
