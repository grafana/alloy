// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package basehash provides basic types to implement hash identifiers.
package basehash // import "go.opentelemetry.io/ebpf-profiler/libpf/basehash"

import (
	"fmt"
	"strconv"
	"strings"
)

// In a variety of different places the profiling agent identifies files or traces with
// an identifier. This identifier consists of two 64-bit integers on the database
// layer, but is really a 128-bit hash.
//
// At the moment, most of the eBPF infrastructure that the profiling agent contains still
// uses 64-bit hashes.
//
// In order to have a good migration path for 64-bit hashes to 128-bit hashes,
// the code defines a "base type" called baseHash of 64 bits here, along with
// various methods for marshaling/unmarshaling to JSON and regular string here.
//
// Hopefully, when the rest of the code is ready, upgrading the types here to
// a struct that contains two uint64s should be easy.

const lowerHex = "0123456789abcdef"
const upperHex = "0123456789ABCDEF"

func putUint64AsHex(n uint64, b []byte, mapping string) {
	b[0] = mapping[(n>>60)&0x0F]
	b[1] = mapping[(n>>56)&0x0F]
	b[2] = mapping[(n>>52)&0x0F]
	b[3] = mapping[(n>>48)&0x0F]
	b[4] = mapping[(n>>44)&0x0F]
	b[5] = mapping[(n>>40)&0x0F]
	b[6] = mapping[(n>>36)&0x0F]
	b[7] = mapping[(n>>32)&0x0F]
	b[8] = mapping[(n>>28)&0x0F]
	b[9] = mapping[(n>>24)&0x0F]
	b[10] = mapping[(n>>20)&0x0F]
	b[11] = mapping[(n>>16)&0x0F]
	b[12] = mapping[(n>>12)&0x0F]
	b[13] = mapping[(n>>8)&0x0F]
	b[14] = mapping[(n>>4)&0x0F]
	b[15] = mapping[n&0x0F]
}

// putUint64AsLowerHex encodes a uint64 into b as a lowercase hexadecimal.
func putUint64AsLowerHex(n uint64, b []byte) {
	putUint64AsHex(n, b, lowerHex)
}

// putUint64AsUpperHex encodes a uint64 into b as an uppercase hexadecimal.
func putUint64AsUpperHex(n uint64, b []byte) {
	putUint64AsHex(n, b, upperHex)
}

func uint64ToString(n uint64) string {
	return strconv.FormatUint(n, 10)
}

func uint64ToGob(n uint64, ch rune) string {
	return fmt.Sprintf("%%!%c(uint64=%s)", ch, uint64ToString(n))
}

func uint64ToLowerHex(n uint64) string {
	return strconv.FormatUint(n, 16)
}

func uint64ToUpperHex(n uint64) string {
	return strings.ToUpper(strconv.FormatUint(n, 16))
}

// These marshaling helper methods assist the Go compiler with inlining while
// still improving readability.
func marshalIdentifierTo(hi, lo uint64, b []byte) {
	putUint64AsLowerHex(hi, b[0:16])
	putUint64AsLowerHex(lo, b[16:32])
}

func marshalQuotedIdentifierTo(hi, lo uint64, b []byte) {
	b[0] = '"'
	marshalIdentifierTo(hi, lo, b[1:])
	b[33] = '"'
}

func marshalIdentifier(hi, lo uint64) []byte {
	buf := make([]byte, 32)
	marshalIdentifierTo(hi, lo, buf)
	return buf
}

func marshalQuotedIdentifier(hi, lo uint64) []byte {
	buf := make([]byte, 34)
	marshalQuotedIdentifierTo(hi, lo, buf)
	return buf
}
