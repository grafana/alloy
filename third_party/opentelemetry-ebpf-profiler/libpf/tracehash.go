// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import (
	"encoding"
	"encoding/base64"

	"go.opentelemetry.io/ebpf-profiler/libpf/basehash"
)

// TraceHash represents the unique hash of a trace
type TraceHash struct {
	basehash.Hash128
}

func NewTraceHash(hi, lo uint64) TraceHash {
	return TraceHash{basehash.New128(hi, lo)}
}

// TraceHashFromBytes parses a byte slice of a trace hash into the internal data representation.
func TraceHashFromBytes(b []byte) (TraceHash, error) {
	h, err := basehash.New128FromBytes(b)
	if err != nil {
		return TraceHash{}, err
	}
	return TraceHash{h}, nil
}

func (h TraceHash) Equal(other TraceHash) bool {
	return h.Hash128.Equal(other.Hash128)
}

func (h TraceHash) Less(other TraceHash) bool {
	return h.Hash128.Less(other.Hash128)
}

// EncodeTo encodes the hash into the base64 encoded representation
// and stores it in the provided destination byte array.
// The length of the destination must be at least EncodedLen().
func (h TraceHash) EncodeTo(dst []byte) {
	base64.RawURLEncoding.Encode(dst, h.Bytes())
}

// EncodedLen returns the length of the hash's base64 representation.
func (TraceHash) EncodedLen() int {
	// TraceHash is 16 bytes long, the base64 representation is one base64 byte per 6 bits.
	return ((16)*8)/6 + 1
}

// Hash32 returns a 32 bits hash of the input.
// It's main purpose is to be used for LRU caching.
func (h TraceHash) Hash32() uint32 {
	return uint32(h.Lo())
}

// Compile-time interface checks
var _ encoding.TextUnmarshaler = (*TraceHash)(nil)
var _ encoding.TextMarshaler = (*TraceHash)(nil)
