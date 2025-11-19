// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golabels // import "go.opentelemetry.io/ebpf-profiler/interpreter/golabels"

import (
	"go/version"

	"go.opentelemetry.io/ebpf-profiler/support"
)

// Offsets come from DWARF debug information, use tools/gooffsets to extract them.
// However since DWARF information can be stripped we record them here.
// TODO: Should we look for DWARF information to support new versions
// automatically when available?
func getOffsets(vers string) support.GoLabelsOffsets {
	offsets := support.GoLabelsOffsets{
		// https://github.com/golang/go/blob/80e2e474b8d9124d03b744f/src/runtime/runtime2.go#L410
		M_offset: 48,
		// https://github.com/golang/go/blob/80e2e474b8d9124d03b744f/src/runtime/runtime2.go#L541
		Curg: 192,
		// https://github.com/golang/go/blob/80e2e474b8d9124d03b744f/src/runtime/runtime2.go#L483
		Labels: 0,
		// https://github.com/golang/go/blob/6885bad7dd86880be6929c0/src/runtime/map.go#L112
		Hmap_count: 0,
		// https://github.com/golang/go/blob/6885bad7dd86880be6929c0/src/runtime/map.go#L114
		Hmap_log2_bucket_count: 0,
		// https://github.com/golang/go/blob/6885bad7dd86880be6929c0/src/runtime/map.go#L118
		Hmap_buckets: 0,
	}

	// Version enforcement takes place in the Loader function.
	if version.Compare(vers, "go1.25") >= 0 {
		offsets.Curg = 184
		offsets.Labels = 344
		return offsets
	} else if version.Compare(vers, "go1.24") >= 0 {
		offsets.Labels = 352
		return offsets
	}

	// These are the same for all versions but we have to leave them zero for 1.24+ detection.
	offsets.Hmap_log2_bucket_count = 9
	offsets.Hmap_buckets = 16
	if version.Compare(vers, "go1.23") >= 0 {
		offsets.Labels = 352
	} else if version.Compare(vers, "go1.21") >= 0 {
		offsets.Labels = 344
	} else if version.Compare(vers, "go1.17") >= 0 {
		offsets.Labels = 360
	} else {
		offsets.Labels = 344
	}
	return offsets
}
