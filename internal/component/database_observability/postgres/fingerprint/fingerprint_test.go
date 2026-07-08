package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint_StableAcrossCommentsAndWhitespace(t *testing.T) {
	a, _, errA := Fingerprint("SELECT * FROM users WHERE id = $1 -- foo", SourceLog)
	require.NoError(t, errA)
	b, _, errB := Fingerprint("SELECT *\nFROM users\nWHERE id = $1 /* bar */", SourceLog)
	require.NoError(t, errB)
	assert.Equal(t, a, b)
}

func TestFingerprint_DifferentForDifferentQueries(t *testing.T) {
	a, _, _ := Fingerprint("SELECT * FROM users", SourceLog)
	b, _, _ := Fingerprint("SELECT * FROM products", SourceLog)
	assert.NotEqual(t, a, b)
}

func TestFingerprint_RepairUnclosedQuotes(t *testing.T) {
	want, _, errWant := Fingerprint("SELECT * FROM t WHERE name = 'oh no'", SourceLog)
	require.NoError(t, errWant)

	fp, repaired, err := Fingerprint("SELECT * FROM t WHERE name = 'oh no", SourceLog)
	require.NoError(t, err)
	assert.True(t, repaired, "should report that repair was used")
	assert.Equal(t, want, fp, "repaired fingerprint must match the closed-quote form")
}

func TestFingerprint_RepairUnclosedParens(t *testing.T) {
	want, _, errWant := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3)", SourceLog)
	require.NoError(t, errWant)

	fp, repaired, err := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3", SourceLog)
	require.NoError(t, err)
	assert.True(t, repaired)
	assert.Equal(t, want, fp, "repaired fingerprint must match the closed-paren form")
}

func TestFingerprint_UnparsableSentinel(t *testing.T) {
	fp, _, err := Fingerprint("$$$ not sql at all $$$", SourceLog)
	require.NoError(t, err)
	assert.Equal(t, FingerprintOf(SentinelUnparsable), fp)
}

func TestFingerprint_EmptyAndNullInputs(t *testing.T) {
	_, _, err := Fingerprint("", SourceLog)
	assert.Error(t, err, "empty input should error so callers can skip emitting")
}

func TestFingerprint_SentinelStability(t *testing.T) {
	t.Run("unparsable sentinel is stable", func(t *testing.T) {
		first, _, _ := Fingerprint("$$$ not sql at all $$$", SourceLog)
		second, _, _ := Fingerprint("$$$ not sql at all $$$", SourceLog)
		assert.Equal(t, first, second)
		assert.Equal(t, FingerprintOf(SentinelUnparsable), first)
	})
}
