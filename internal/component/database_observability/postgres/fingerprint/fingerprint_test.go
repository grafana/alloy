//go:build cgo

package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint_StableAcrossCommentsAndWhitespace(t *testing.T) {
	a, errA := Fingerprint("SELECT * FROM users WHERE id = $1 -- foo")
	require.NoError(t, errA)
	b, errB := Fingerprint("SELECT *\nFROM users\nWHERE id = $1 /* bar */")
	require.NoError(t, errB)
	assert.Equal(t, a, b)
}

func TestFingerprint_DifferentForDifferentQueries(t *testing.T) {
	a, _ := Fingerprint("SELECT * FROM users")
	b, _ := Fingerprint("SELECT * FROM products")
	assert.NotEqual(t, a, b)
}

func TestFingerprint_RepairUnclosedQuotes(t *testing.T) {
	want, errWant := Fingerprint("SELECT * FROM t WHERE name = 'oh no'")
	require.NoError(t, errWant)

	fp, err := Fingerprint("SELECT * FROM t WHERE name = 'oh no")
	require.NoError(t, err)
	assert.Equal(t, want, fp, "repaired fingerprint must match the closed-quote form")
}

func TestFingerprint_RepairUnclosedParens(t *testing.T) {
	want, errWant := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3)")
	require.NoError(t, errWant)

	fp, err := Fingerprint("SELECT * FROM t WHERE id IN (1, 2, 3")
	require.NoError(t, err)
	assert.Equal(t, want, fp, "repaired fingerprint must match the closed-paren form")
}

func TestFingerprint_UnparsableSentinel(t *testing.T) {
	fp, err := Fingerprint("$$$ not sql at all $$$")
	require.NoError(t, err)
	assert.Equal(t, FingerprintOf(SentinelUnparsable), fp)
}

func TestFingerprint_EmptyAndNullInputs(t *testing.T) {
	_, err := Fingerprint("")
	assert.Error(t, err, "empty input should error so callers can skip emitting")
}

func TestFingerprint_SentinelStability(t *testing.T) {
	t.Run("unparsable sentinel is stable", func(t *testing.T) {
		first, _ := Fingerprint("$$$ not sql at all $$$")
		second, _ := Fingerprint("$$$ not sql at all $$$")
		assert.Equal(t, first, second)
		assert.Equal(t, FingerprintOf(SentinelUnparsable), first)
	})
}
