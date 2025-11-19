// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util // import "go.opentelemetry.io/ebpf-profiler/util"

import (
	"math/bits"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"go.opentelemetry.io/ebpf-profiler/libpf/hash"
)

// IsValidString checks if string is UTF-8-encoded and only contains expected characters.
func IsValidString(s string) bool {
	if s == "" {
		return false
	}
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// NextPowerOfTwo returns input value if it's a power of two,
// otherwise it returns the next power of two.
func NextPowerOfTwo(v uint32) uint32 {
	if v == 0 {
		return 1
	}
	return 1 << bits.Len32(v-1)
}

// AtomicUpdateMaxUint32 updates the value in store using atomic memory primitives. newValue will
// only be placed in store if newValue is larger than the current value in store.
// To avoid inconsistency parallel updates to store should be avoided.
func AtomicUpdateMaxUint32(store *atomic.Uint32, newValue uint32) {
	for {
		// Load the current value
		oldValue := store.Load()
		if newValue <= oldValue {
			// No update needed.
			break
		}
		if store.CompareAndSwap(oldValue, newValue) {
			// The value was atomically updated.
			break
		}
		// The value changed between load and update attempt.
		// Retry with the new value.
	}
}

// VersionUint returns a single integer composed of major, minor, patch.
func VersionUint(major, minor, patch uint32) uint32 {
	return (major << 16) + (minor << 8) + patch
}

// Range describes a range with Start and End values.
type Range struct {
	Start uint64
	End   uint64
}

// OnDiskFileIdentifier can be used as unique identifier for a file.
// It is a structure to identify a particular file on disk by
// deviceID and inode number.
type OnDiskFileIdentifier struct {
	DeviceID uint64 // dev_t as reported by stat.
	InodeNum uint64 // ino_t should fit into 64 bits
}

func (odfi OnDiskFileIdentifier) Hash32() uint32 {
	return uint32(hash.Uint64(odfi.InodeNum) + odfi.DeviceID)
}
