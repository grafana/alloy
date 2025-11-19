// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"errors"
	"io"
)

// nibbleReader provides the interface to read nibble encoded data as implemented in
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblestream.h#L188
type nibbleReader struct {
	io.ByteReader

	cachedNibble uint8

	err error
}

func (nr *nibbleReader) Error() error {
	return nr.err
}

func (nr *nibbleReader) AlignToBytes() {
	nr.cachedNibble = 0
}

// ReadNibble reads one nibble from the stream.
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblestream.h#L213
func (nr *nibbleReader) ReadNibble() uint8 {
	if nr.err != nil {
		return 0
	}
	if nr.cachedNibble != 0 {
		nibble := nr.cachedNibble & 0xf
		nr.cachedNibble = 0
		return nibble
	}

	b, err := nr.ReadByte()
	if err != nil {
		nr.err = err
		return 0
	}

	// Lower nibble first
	nibble := b & 0xf
	nr.cachedNibble = 0xf0 | (b >> 4)
	return nibble
}

// Uint32 reads one nibble encoded 32-bit unsigned integer.
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblestream.h#L250
func (nr *nibbleReader) Uint32() uint32 {
	val := uint32(0)
	for range 11 {
		n := nr.ReadNibble()
		val = (val << 3) + uint32(n&0x7)
		if n&0x8 == 0 {
			return val
		}
	}
	nr.err = errors.New("corrupt nibble data")
	return 0
}

// Int32 reads one nibble encoded 32-bit signed integer.
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblestream.h#L292
func (nr *nibbleReader) Int32() int32 {
	raw := nr.Uint32()
	val := int32(raw >> 1)
	if raw&1 != 0 {
		val = -val
	}
	return val
}

// Ptr reads one raw pointer value.
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/nibblestream.h#L307
// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/debuginfostore.cpp#L224
func (nr *nibbleReader) Ptr() uint64 {
	val := uint64(0)
	for i := 0; i < 64; i += 4 {
		n := nr.ReadNibble()
		val |= uint64(n) << i
	}
	return val
}
