// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nopanicslicereader

import (
	"testing"

	"go.opentelemetry.io/ebpf-profiler/libpf"

	"github.com/stretchr/testify/assert"
)

func TestSliceReader(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	assert.Equal(t, uint16(0x0403), Uint16(data, 2))
	assert.Equal(t, uint16(0), Uint16(data, 7))
	assert.Equal(t, uint32(0x04030201), Uint32(data, 0))
	assert.Equal(t, uint32(0), Uint32(data, 100))
	assert.Equal(t, uint64(0x0807060504030201), Uint64(data, 0))
	assert.Equal(t, uint64(0), Uint64(data, 1))
	assert.Equal(t, libpf.Address(0x0807060504030201), Ptr(data, 0))
	assert.Equal(t, libpf.Address(0x08070605), PtrDiff32(data, 4))
}
