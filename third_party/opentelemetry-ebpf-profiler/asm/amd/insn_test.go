// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package amd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndBr64(t *testing.T) {
	res, n := DecodeSkippable([]byte{0xF3, 0x0F, 0x1E, 0xFA})
	assert.True(t, res)
	assert.Equal(t, 4, n)

	res, _ = DecodeSkippable([]byte{})
	assert.False(t, res)
}
