// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaLineNumbers tests that the Hotspot delta encoded line table decoding works.
// The set here is an actually table extracting from JVM. It is fairly easy to encode
// these numbers if needed, but we don't need to generate them currently for anything.
func TestJavaLineNumbers(t *testing.T) {
	bciLine := []struct {
		bci, line uint32
	}{
		{0, 478},
		{5, 479},
		{9, 480},
		{19, 481},
		{26, 482},
		{33, 483},
		{47, 490},
		{50, 485},
		{52, 486},
		{58, 490},
		{61, 488},
		{63, 489},
		{68, 491},
	}

	decoder := unsigned5Decoder{
		r: bytes.NewReader([]byte{
			255, 0, 252, 11, 41, 33, 81, 57, 57, 119,
			255, 6, 9, 17, 52, 255, 6, 3, 17, 42, 0}),
	}

	var bci, line uint32
	for i := range bciLine {
		err := decoder.decodeLineTableEntry(&bci, &line)
		require.NoError(t, err)
		assert.Equal(t, bciLine[i].bci, bci)
		assert.Equal(t, bciLine[i].line, line)
	}
	err := decoder.decodeLineTableEntry(&bci, &line)
	assert.ErrorIs(t, err, io.EOF, "line table not empty at end")
}
