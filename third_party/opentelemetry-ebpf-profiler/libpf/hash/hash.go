// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package hash provides the same hash primitives as used by the eBPF.
// This file should be kept in sync with the eBPF tracemgmt.h.
package hash // import "go.opentelemetry.io/ebpf-profiler/libpf/hash"

// Uint32 computes a hash of a 32-bit uint using the finalizer function for Murmur.
// 32-bit via https://en.wikipedia.org/wiki/MurmurHash#Algorithm
func Uint32(x uint32) uint32 {
	x ^= x >> 16
	x *= 0x85ebca6b
	x ^= x >> 13
	x *= 0xc2b2ae35
	x ^= x >> 16
	return x
}

// Uint64 computes a hash of a 64-bit uint using the finalizer function for Murmur3
// Via https://lemire.me/blog/2018/08/15/fast-strongly-universal-64-bit-hashing-everywhere/
func Uint64(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}
