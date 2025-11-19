// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basehash

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromBytes(t *testing.T) {
	_, err := New128FromBytes(nil)
	require.Error(t, err)

	b := []byte{}
	_, err = New128FromBytes(b)
	require.Error(t, err)

	b = []byte{1}
	_, err = New128FromBytes(b)
	require.Error(t, err)

	b = []byte{0, 1, 2, 3, 4, 5, 6, 7}
	_, err = New128FromBytes(b)
	require.Error(t, err)

	b = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	hash, err := New128FromBytes(b)
	require.NoError(t, err)
	assert.Equal(t, New128(0x01020304050607, 0x08090A0B0C0D0E0F), hash)
}

func TestEqual(t *testing.T) {
	hash := New128(0xDEC0DE, 0xC0FFEE)

	assert.True(t, hash.Equal(New128(0xDEC0DE, 0xC0FFEE)))
	assert.False(t, hash.Equal(New128(0xDEC0DE, 0)))
	assert.False(t, hash.Equal(New128(0, 0xC0FFEE)))
	assert.False(t, hash.Equal(New128(0xDECADE, 0xCAFE)))
}

func TestLess(t *testing.T) {
	// left.hi == right.hi and left.lo < right.lo
	a, b := New128(0, 1), New128(0, 2)
	assert.True(t, a.Less(b))

	// left.hi == right.hi and left.lo > right.lo
	c, d := New128(0, 2), New128(0, 1)
	assert.False(t, c.Less(d))

	// left.hi == right.hi and left.lo == right.lo
	e, f := New128(0, 2), New128(0, 2)
	assert.False(t, e.Less(f))

	// left.hi < right.hi
	g, h := New128(0, 0), New128(1, 1)
	assert.True(t, g.Less(h))

	// left.hi > right.hi
	i, j := New128(1, 1), New128(0, 0)
	assert.False(t, i.Less(j))
}

func TestIsZero(t *testing.T) {
	assert.True(t, New128(0, 0).IsZero())
	assert.False(t, New128(5550100, 0).IsZero())
	assert.False(t, New128(0, 5550100).IsZero())
	assert.False(t, New128(5550100, 5550100).IsZero())
}

func TestBytes(t *testing.T) {
	testCases := []struct {
		name     string
		hash     Hash128
		expected []byte
	}{
		{
			name:     "Zero hash",
			hash:     New128(0, 0),
			expected: make([]byte, 16),
		},
		{
			name:     "Non-zero hash",
			hash:     New128(0xDEC0DE, 0xC0FFEE),
			expected: []byte{0, 0, 0, 0, 0, 0xDE, 0xC0, 0xDE, 0, 0, 0, 0, 0, 0xC0, 0xFF, 0xEE},
		},
		{
			name:     "Non-zero low bits",
			hash:     New128(0, 0xC0FFEE),
			expected: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xC0, 0xFF, 0xEE},
		},
		{
			name: "Max uint64",
			hash: New128(math.MaxUint64, math.MaxUint64),
			expected: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.hash.Bytes())
		})
	}
}

func TestPutBytes16(t *testing.T) {
	var b [16]byte
	hash := New128(0x0011223344556677, 0x8899AABBCCDDEEFF)
	hash.PutBytes16(&b)

	expected := []byte{
		0x00, 0x11, 0x22, 0x33,
		0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xAA, 0xBB,
		0xCC, 0xDD, 0xEE, 0xFF,
	}

	assert.Equal(t, expected, hash.Bytes())
}

func TestHash128Format(t *testing.T) {
	h, _ := New128FromBytes([]byte{
		0x00, 0x11, 0x22, 0x33,
		0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xAA, 0xBB,
		0xCC, 0xDD, 0xEE, 0xFF})

	tests := map[string]struct {
		formatter string
		expected  string
	}{
		"v":  {formatter: "%v", expected: "{4822678189205111 9843086184167632639}"},
		"d":  {formatter: "%d", expected: "{4822678189205111 9843086184167632639}"},
		"x":  {formatter: "%x", expected: "112233445566778899aabbccddeeff"},
		"X":  {formatter: "%X", expected: "112233445566778899AABBCCDDEEFF"},
		"#v": {formatter: "%#v", expected: "0x112233445566778899aabbccddeeff"},
		"#x": {formatter: "%#x", expected: "0x112233445566778899aabbccddeeff"},
		"#X": {formatter: "%#X", expected: "0x112233445566778899AABBCCDDEEFF"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			output := fmt.Sprintf(test.formatter, h)
			assert.Equal(t, test.expected, output)
		})
	}
}

func TestHash128StringNoQuotes(t *testing.T) {
	id := New128(0x0011223344556677, 0x8899AABBCCDDEEFF)
	assert.Equal(t, "00112233445566778899aabbccddeeff", id.StringNoQuotes())
}

func TestNew128FromString(t *testing.T) {
	tests := map[string]struct {
		stringRepresentation string
		expected             Hash128
		err                  error
	}{
		"hex": {stringRepresentation: "97b7371e9fc83bc7b9ab5ee193a98020",
			expected: Hash128{hi: 10932267225134414791, lo: 13378891440972202016}},
		"hex with prefix": {stringRepresentation: "0x97b7371e9fc83bc7b9ab5ee193a98020",
			expected: Hash128{hi: 10932267225134414791, lo: 13378891440972202016}},
		"uuid": {stringRepresentation: "97b7371e-9fc8-3bc7-b9ab-5ee193a98020",
			expected: Hash128{hi: 10932267225134414791, lo: 13378891440972202016}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := New128FromString(tc.stringRepresentation)
			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.expected, got)
		})
	}
}
