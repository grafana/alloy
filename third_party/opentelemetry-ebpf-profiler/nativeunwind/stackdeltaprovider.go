// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nativeunwind // import "go.opentelemetry.io/ebpf-profiler/nativeunwind"

import (
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
)

type Statistics struct {
	// Number of times of successful extractions.
	Success uint64

	// Number of times extracting stack deltas failed.
	ExtractionErrors uint64
}

// StackDeltaProvider defines an interface for types that provide access to the stack deltas from
// executables.
type StackDeltaProvider interface {
	// GetIntervalStructuresForFile inspects a single executable and extracts data that is needed
	// to rebuild the stack for traces of this executable.
	GetIntervalStructuresForFile(elfRef *pfelf.Reference, interval *sdtypes.IntervalData) error

	// GetAndResetStatistics returns the internal statistics for this provider and resets all
	// values to 0.
	GetAndResetStatistics() Statistics
}
