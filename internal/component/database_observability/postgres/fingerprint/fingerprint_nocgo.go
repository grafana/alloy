//go:build !cgo

package fingerprint

import "errors"

// Source records where a query text came from. See the cgo build for details.
type Source int

const (
	SourcePgStatStatements Source = iota
	SourceLog
)

const SentinelUnparsable = "<unparsable query>"

var ErrEmpty = errors.New("fingerprint: empty query text")

// Fingerprint is a no-op under !cgo. libpg_query requires cgo; the cross-
// compile target (CGO_ENABLED=0) gets stubs so the rest of the codebase
// builds without it. Callers treat err != nil as "no fingerprint, skip emit".
func Fingerprint(query string, source Source) (string, bool, error) {
	return "", false, ErrEmpty
}

func FingerprintOf(text string) string { return "" }

func SentinelKind(fp string) string { return "" }
