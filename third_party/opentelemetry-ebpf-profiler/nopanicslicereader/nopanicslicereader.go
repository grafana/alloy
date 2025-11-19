// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// nopanicslicereader provides little convenience utilities to read "native" endian
// values from a slice at given offset. Zeroes are returned on out of bounds access
// instead of panic.
package nopanicslicereader // import "go.opentelemetry.io/ebpf-profiler/nopanicslicereader"

import (
	"encoding/binary"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// Uint8 reads one 8-bit unsigned integer from given byte slice offset
func Uint8(b []byte, offs uint) uint8 {
	if offs+1 > uint(len(b)) {
		return 0
	}
	return b[offs]
}

// Uint16 reads one 16-bit unsigned integer from given byte slice offset
func Uint16(b []byte, offs uint) uint16 {
	if offs+2 > uint(len(b)) {
		return 0
	}
	return binary.LittleEndian.Uint16(b[offs:])
}

// Uint32 reads one 32-bit unsigned integer from given byte slice offset
func Uint32(b []byte, offs uint) uint32 {
	if offs+4 > uint(len(b)) {
		return 0
	}
	return binary.LittleEndian.Uint32(b[offs:])
}

// Int32 reads one 32-bit signed integer from given byte slice offset
func Int32(b []byte, offs uint) int32 {
	if offs+4 > uint(len(b)) {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(b[offs:]))
}

// Uint64 reads one 64-bit unsigned integer from given byte slice offset
func Uint64(b []byte, offs uint) uint64 {
	if offs+8 > uint(len(b)) {
		return 0
	}
	return binary.LittleEndian.Uint64(b[offs:])
}

// Ptr reads one native sized pointer from given byte slice offset
func Ptr(b []byte, offs uint) libpf.Address {
	return libpf.Address(Uint64(b, offs))
}

// PtrDiff16 reads one 16-bit unsigned integer from given byte slice offset
// and returns it as an address
func PtrDiff16(b []byte, offs uint) libpf.Address {
	return libpf.Address(Uint16(b, offs))
}

// PtrDiff32 reads one 32-bit unsigned integer from given byte slice offset
// and returns it as an address
func PtrDiff32(b []byte, offs uint) libpf.Address {
	return libpf.Address(Uint32(b, offs))
}
