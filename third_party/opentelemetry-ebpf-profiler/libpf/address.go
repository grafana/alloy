// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import "go.opentelemetry.io/ebpf-profiler/libpf/hash"

// Address represents an address, or offset within a process
type Address uintptr

// Hash32 returns a 32 bits hash of the input.
// It's main purpose is to be used as key for caching.
func (adr Address) Hash32() uint32 {
	return uint32(adr.Hash())
}

// Hash returns a 64 bits hash of the input.
func (adr Address) Hash() uint64 {
	return hash.Uint64(uint64(adr))
}
