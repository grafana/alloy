// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package times // import "go.opentelemetry.io/ebpf-profiler/times"

import (
	"time"
	_ "unsafe" // required to use //go:linkname for runtime.nanotime
)

// KTime stores a time value, retrieved from a monotonic clock, in nanoseconds
type KTime int64

// GetKTime gets the current time in same nanosecond format as bpf_ktime_get_ns() eBPF call
// This relies runtime.nanotime to use CLOCK_MONOTONIC. If this changes, this needs to
// be adjusted accordingly. Using this internal is superior in performance, as it is able
// to use the vDSO to query the time without syscall.
//
//go:noescape
//go:linkname GetKTime runtime.nanotime
func GetKTime() KTime

// Time converts the kernel timestamp into a Go time object.
func (t KTime) Time() time.Time {
	return time.Unix(0, t.UnixNano())
}

// UnixNano converts the kernel timestamp to nanoseconds since the epoch.
func (t KTime) UnixNano() int64 {
	return int64(t) + bootTimeUnixNano.Load()
}
