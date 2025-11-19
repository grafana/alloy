// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordingReader(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	rr := newRecordingReader(bytes.NewReader(data), 0, 2)
	for i := 0; i < len(data)-1; i++ {
		b, err := rr.ReadByte()
		require.NoError(t, err)
		assert.Equal(t, data[i], b)
	}
	assert.Len(t, rr.GetBuffer(), len(data)-1)
}
