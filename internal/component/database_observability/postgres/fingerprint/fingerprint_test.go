package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint_StableAcrossCommentsAndWhitespace(t *testing.T) {
	a, _, errA := Fingerprint("SELECT * FROM users WHERE id = $1 -- foo", SourcePgStatStatements, 0)
	require.NoError(t, errA)
	b, _, errB := Fingerprint("SELECT *\nFROM users\nWHERE id = $1 /* bar */", SourcePgStatStatements, 0)
	require.NoError(t, errB)
	assert.Equal(t, a, b)
}

func TestFingerprint_DifferentForDifferentQueries(t *testing.T) {
	// Use structurally different queries — SQL fingerprinting normalises
	// constants, so "SELECT 1" vs "SELECT 2" hash to the same value.
	a, _, _ := Fingerprint("SELECT * FROM users", SourcePgStatStatements, 0)
	b, _, _ := Fingerprint("SELECT * FROM products", SourcePgStatStatements, 0)
	assert.NotEqual(t, a, b)
}

func TestFingerprint_RepairUnclosedQuotes(t *testing.T) {
	fp, repaired, err := Fingerprint("SELECT * FROM t WHERE name = 'oh no", SourceLog, 0)
	require.NoError(t, err)
	assert.True(t, repaired, "should report that repair was used")
	assert.NotEqual(t, "", fp)
}

func TestFingerprint_RepairUnclosedParens(t *testing.T) {
	fp, repaired, err := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3", SourceLog, 0)
	require.NoError(t, err)
	assert.True(t, repaired)
	assert.NotEqual(t, "", fp)
}

func TestFingerprint_TruncatedSentinelOnPgStatActivity(t *testing.T) {
	const trackSize = 1024
	bad := makeUnparsableOfLen(trackSize - 1)

	fp, _, err := Fingerprint(bad, SourcePgStatActivity, trackSize)
	require.NoError(t, err)
	assert.Equal(t, FingerprintOf(SentinelTruncated), fp)
}

func TestFingerprint_UnparsableSentinel(t *testing.T) {
	fp, _, err := Fingerprint("$$$ not sql at all $$$", SourcePgStatStatements, 0)
	require.NoError(t, err)
	assert.Equal(t, FingerprintOf(SentinelUnparsable), fp)
}

func TestFingerprint_EmptyAndNullInputs(t *testing.T) {
	_, _, err := Fingerprint("", SourcePgStatStatements, 0)
	assert.Error(t, err, "empty input should error so callers can skip emitting")
}

// makeUnparsableOfLen returns a string of exactly n bytes that is invalid SQL
// and remains unparseable even after repair() closes quotes/parentheses. This
// is needed because repair() can salvage a truncated IN-list (SELECT … IN
// (1,1,…) is valid once the paren is closed) so that path never reaches the
// sentinel branch.
func makeUnparsableOfLen(n int) string {
	s := "NOT VALID SQL !!! "
	for len(s) < n {
		s += "x "
	}
	return s[:n]
}
