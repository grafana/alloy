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

func TestNativeReader(t *testing.T) {
	testCases := []struct {
		data     string
		expected uint32
	}{
		{"18", 12},
		{"a10f", 1000},
	}

	for _, test := range testCases {
		t.Run(test.data, func(t *testing.T) {
			data, err := hex.DecodeString(test.data)
			require.NoError(t, err, "Hex decoding failed")

			decoder := nativeReader{
				ReaderAt: bytes.NewReader(data),
			}
			value, _, err := decoder.Uint(0)
			require.NoError(t, err, "Error")
			assert.Equal(t, test.expected, value, "Wrong native decoding")
		})
	}
}
