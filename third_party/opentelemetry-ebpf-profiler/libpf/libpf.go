// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import (
	"encoding/json"
	"time"
)

// UnixTime32 is another type to represent seconds since epoch.
// In most cases 32bit time values are good enough until year 2106.
// Our time series database backend uses this type for TimeStamps as well,
// so there is no need to use a different type than uint32.
// Also, Go's semantics on map[time.Time] are particularly nasty footguns,
// and since the code is mostly dealing with UNIX timestamps, we may
// as well use uint32s instead.
// To restore some semblance of type safety, we declare a type alias here.
type UnixTime32 uint32

func (t UnixTime32) MarshalJSON() ([]byte, error) {
	return time.Unix(int64(t), 0).UTC().MarshalJSON()
}

// Compile-time interface checks
var _ json.Marshaler = (*UnixTime32)(nil)

// NowAsUInt32 is a convenience function to avoid code repetition
func NowAsUInt32() uint32 {
	return uint32(time.Now().Unix())
}

// UnixTime64 represents nanoseconds since epoch.
type UnixTime64 uint64

// AddressOrLineno represents a line number in an interpreted file or an offset into
// a native file.
type AddressOrLineno uint64

// Void allows to use maps as sets without memory allocation for the values.
// From the "Go Programming Language":
//
//	The struct type with no fields is called the empty struct, written struct{}. It has size zero
//	and carries no information but may be useful nonetheless. Some Go programmers
//	use it instead of bool as the value type of a map that represents a set, to emphasize
//	that only the keys are significant, but the space saving is marginal and the syntax more
//	cumbersome, so we generally avoid it.
type Void struct{}

// SourceLineno represents a line number within a source file. It is intended to be used for the
// source line numbers associated with offsets in native code, or for source line numbers in
// interpreted code.
type SourceLineno uint64

// Origin determines the source of a trace.
type Origin int
