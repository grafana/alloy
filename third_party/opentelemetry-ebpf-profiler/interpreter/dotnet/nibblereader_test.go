// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNibbleReader(t *testing.T) {
	testCases := []struct {
		data     string
		expected uint32
	}{
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblestream.h#L32-L46
		{"00", 0},
		{"71", 1},
		{"07", 7},
		{"09", 8},
		{"19", 9},
		{"7f", 63},
		{"8900", 64}, // incorrect example in dotnet code
		{"8901", 65}, // incorrect example in dotnet code
		{"ff07", 511},
		{"8908", 512},
		{"8918", 513},
	}

	for _, test := range testCases {
		t.Run(test.data, func(t *testing.T) {
			data, err := hex.DecodeString(test.data)
			require.NoError(t, err, "Hex decoding failed")

			decoder := nibbleReader{
				ByteReader: bytes.NewReader(data),
			}
			value := decoder.Uint32()
			require.NoError(t, decoder.Error(), "Error")
			assert.Equal(t, test.expected, value, "Wrong nibble decoding")
		})
	}
}
