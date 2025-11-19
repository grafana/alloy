// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// remotememory provides access to memory space of a process. The ReaderAt
// interface is used for the basic access, and various convenience functions are
// provided to help reading specific data types.
package remotememory // import "go.opentelemetry.io/ebpf-profiler/remotememory"

import (
	"bytes"
	"encoding/binary"
	"io"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// RemoteMemory implements a set of convenience functions to access the remote memory
type RemoteMemory struct {
	io.ReaderAt
	// Bias is the adjustment for pointers (used to unrelocate pointers in coredump)
	Bias libpf.Address
}

// Valid determines if this RemoteMemory instance contains a valid reference to target process
func (rm RemoteMemory) Valid() bool {
	return rm.ReaderAt != nil
}

// Read fills slice p[] with data from remote memory at address addr
func (rm RemoteMemory) Read(addr libpf.Address, p []byte) error {
	_, err := rm.ReadAt(p, int64(addr))
	return err
}

// Ptr reads a native pointer from remote memory
func (rm RemoteMemory) Ptr(addr libpf.Address) libpf.Address {
	var buf [8]byte
	if rm.Read(addr, buf[:]) != nil {
		return 0
	}
	return libpf.Address(binary.LittleEndian.Uint64(buf[:])) - rm.Bias
}

// Uint8 reads an 8-bit unsigned integer from remote memory
func (rm RemoteMemory) Uint8(addr libpf.Address) uint8 {
	var buf [1]byte
	if rm.Read(addr, buf[:]) != nil {
		return 0
	}
	return buf[0]
}

// Uint16 reads a 16-bit unsigned integer from remote memory
func (rm RemoteMemory) Uint16(addr libpf.Address) uint16 {
	var buf [2]byte
	if rm.Read(addr, buf[:]) != nil {
		return 0
	}
	return binary.LittleEndian.Uint16(buf[:])
}

// Uint32 reads a 32-bit unsigned integer from remote memory
func (rm RemoteMemory) Uint32(addr libpf.Address) uint32 {
	var buf [4]byte
	if rm.Read(addr, buf[:]) != nil {
		return 0
	}
	return binary.LittleEndian.Uint32(buf[:])
}

// Uint64 reads a 64-bit unsigned integer from remote memory
func (rm RemoteMemory) Uint64(addr libpf.Address) uint64 {
	var buf [8]byte
	if rm.Read(addr, buf[:]) != nil {
		return 0
	}
	return binary.LittleEndian.Uint64(buf[:])
}

// String reads a zero terminated string from remote memory
func (rm RemoteMemory) String(addr libpf.Address) string {
	buf := make([]byte, 1024)
	n, err := rm.ReadAt(buf, int64(addr))
	if n == 0 || (err != nil && err != io.EOF) {
		return ""
	}
	buf = buf[:n]
	zeroIdx := bytes.IndexByte(buf, 0)
	if zeroIdx >= 0 {
		return string(buf[:zeroIdx])
	}
	if n != cap(buf) {
		return ""
	}

	bigBuf := make([]byte, 4096)
	copy(bigBuf, buf)
	n, err = rm.ReadAt(bigBuf[len(buf):], int64(addr)+int64(len(buf)))
	if n == 0 || (err != nil && err != io.EOF) {
		return ""
	}
	bigBuf = bigBuf[:len(buf)+n]
	zeroIdx = bytes.IndexByte(bigBuf, 0)
	if zeroIdx >= 0 {
		return string(bigBuf[:zeroIdx])
	}

	// Not a zero terminated string
	return ""
}

// StringPtr reads a zero terminate string by first dereferencing a string pointer
// from target memory
func (rm RemoteMemory) StringPtr(addr libpf.Address) string {
	addr = rm.Ptr(addr)
	if addr == 0 {
		return ""
	}
	return rm.String(addr)
}

// ProcessVirtualMemory implements ReaderAt by using process_vm_readv syscalls
// to read the remote memory.
type ProcessVirtualMemory struct {
	pid libpf.PID
}

// NewProcessVirtualMemory returns RemoteMemory with ProcessVirtualMemory as the underlying reader
func NewProcessVirtualMemory(pid libpf.PID) RemoteMemory {
	return RemoteMemory{ReaderAt: ProcessVirtualMemory{pid}}
}
