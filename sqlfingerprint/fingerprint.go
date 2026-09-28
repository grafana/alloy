// Package sqlfingerprint correlates SQL text with database statement statistics.
// It has no dependency on Alloy, OpenTelemetry, a database connection, or CGO.
// Both producers and consumers of a fingerprint must use the same version and
// Options. A fingerprint identifies a query shape, not an execution or a plan.
package sqlfingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Version identifies the canonicalization and hash format. Changes to query
// equivalence rules must increment this value, even if the API stays the same.
const Version = "v4"

// Dialect selects the lexical and database statistics normalization rules.
type Dialect string

const (
	PostgreSQL Dialect = "postgresql"
	MySQL      Dialect = "mysql"
	SQLServer  Dialect = "microsoft.sql_server"
)

// ParseDialect accepts current and legacy OpenTelemetry db.system values.
func ParseDialect(system string) (Dialect, bool) {
	switch strings.ToLower(system) {
	case "postgresql", "postgres":
		return PostgreSQL, true
	case "mysql":
		return MySQL, true
	case "microsoft.sql_server", "mssql":
		return SQLServer, true
	default:
		return "", false
	}
}

// Options describes SQL parsing modes. Zero values select database defaults.
// Configure a separate processor for sources using different parsing modes.
type Options struct {
	// PostgreSQLVersion is 16, 17, or 18. Zero selects 18. Version 18 collapses
	// constant IN lists in pg_stat_statements; older versions retain their arity.
	PostgreSQLVersion int
	// PostgreSQLBackslashEscapes enables standard_conforming_strings=off.
	PostgreSQLBackslashEscapes bool
	MySQLANSIQuotes            bool
	MySQLNoBackslashEscapes    bool
	// SQLServerQuotedIdentifierOff makes double quotes delimit string literals.
	SQLServerQuotedIdentifierOff bool
	// MaxQueryBytes bounds work before lexing. Zero selects 1 MiB.
	MaxQueryBytes int
}

// Reason is a bounded, SQL-free diagnostic suitable for metric labels.
type Reason string

const (
	InvalidSQL  Reason = "invalid_sql"
	Unsupported Reason = "unsupported"
	Truncated   Reason = "truncated"
	Limit       Reason = "limit"
)

// Failure describes a skipped statement. Statement is its zero-based position
// among nonempty statements, including skipped statements. Diagnostics never
// contain SQL text, since queries can contain secrets.
type Failure struct {
	Statement int
	Reason    Reason
}

// Result contains distinct fingerprints in first-occurrence order. Successful
// statements are retained when another statement fails. An unterminated quote
// or procedural body stops processing because later boundaries are ambiguous.
type Result struct {
	Fingerprints []string
	Failures     []Failure
}

// Fingerprinter is immutable and safe for concurrent use. All per-query state
// is local to Fingerprint, so it can also be shared by backend request handlers.
type Fingerprinter struct {
	options Options
}

// New validates options once, before accepting telemetry or backend requests.
func New(options Options) (*Fingerprinter, error) {
	if options.PostgreSQLVersion == 0 {
		options.PostgreSQLVersion = 18
	}
	if options.PostgreSQLVersion < 16 || options.PostgreSQLVersion > 18 {
		return nil, fmt.Errorf("postgresql_version must be 16, 17, or 18")
	}
	if options.MaxQueryBytes == 0 {
		options.MaxQueryBytes = 1 << 20
	}
	if options.MaxQueryBytes < 1 {
		return nil, fmt.Errorf("max_query_bytes must be positive")
	}
	return &Fingerprinter{options: options}, nil
}

// Fingerprint splits a batch outside strings, identifiers, and comments, then
// normalizes and hashes each supported statement. It never queries a database.
// Unsupported syntax is not repaired or hashed as a shared fallback value.
func (f *Fingerprinter) Fingerprint(dialect Dialect, query string) Result {
	var result Result
	if dialect != PostgreSQL && dialect != MySQL && dialect != SQLServer {
		return Result{Failures: []Failure{{Reason: Unsupported}}}
	}
	if len(query) > f.options.MaxQueryBytes {
		return Result{Failures: []Failure{{Reason: Limit}}}
	}
	if !utf8.ValidString(query) || strings.ContainsRune(query, 0) {
		return Result{Failures: []Failure{{Reason: InvalidSQL}}}
	}
	l := lexer{input: query, dialect: dialect, options: f.options}
	seen := make(map[string]struct{})
	for statement := 0; ; statement++ {
		tokens, reason, done := l.statement()
		if len(tokens) == 0 && reason == "" {
			break
		}
		if statement >= 256 {
			result.Failures = append(result.Failures, Failure{statement, Limit})
			break
		}
		if unsafeBatch(tokens) {
			reason, done = Unsupported, true
		}
		if reason == "" {
			tokens, reason = normalize(tokens, dialect, f.options)
		}
		if reason != "" {
			result.Failures = append(result.Failures, Failure{statement, reason})
		} else {
			fingerprint := hash(dialect, tokens)
			if _, exists := seen[fingerprint]; !exists {
				result.Fingerprints = append(result.Fingerprints, fingerprint)
				seen[fingerprint] = struct{}{}
			}
		}
		if done {
			break
		}
	}
	return result
}

// hash frames tokens by kind and byte length to avoid concatenation collisions.
// The dialect and protocol version are visible and included in the hash domain.
func hash(dialect Dialect, tokens []token) string {
	prefix := Version + ":" + string(dialect) + ":"
	h := sha256.New()
	_, _ = h.Write([]byte(prefix))
	var length [4]byte
	for _, t := range tokens {
		_, _ = h.Write([]byte{byte(t.kind)})
		binary.BigEndian.PutUint32(length[:], uint32(len(t.text)))
		_, _ = h.Write(length[:])
		_, _ = h.Write([]byte(t.text))
	}
	return prefix + hex.EncodeToString(h.Sum(nil))
}
