// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ebpfapi // import "go.opentelemetry.io/ebpf-profiler/processmanager/ebpfapi"

import (
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/util"
)

// EbpfHandler provides the functionality to interact with eBPF maps.
type EbpfHandler interface {
	// Embed interpreter.EbpfHandler as subset of this interface.
	interpreter.EbpfHandler

	// RemoveReportedPID removes a PID from the reported_pids eBPF map.
	RemoveReportedPID(pid libpf.PID)

	// UpdateUnwindInfo writes UnwindInfo to given unwind info array index
	UpdateUnwindInfo(index uint16, info stackdeltatypes.UnwindInfo) error

	// UpdateExeIDToStackDeltas defines a function that updates the eBPF map exe_id_to_stack_deltas
	// for host.FileID with the elements of StackDeltaEBPF. It returns the mapID used.
	UpdateExeIDToStackDeltas(fileID host.FileID, deltas []StackDeltaEBPF) (uint16, error)

	// DeleteExeIDToStackDeltas defines a function that removes the entries from the outer eBPF
	// map exe_id_to_stack_deltas and its associated inner map entries.
	DeleteExeIDToStackDeltas(fileID host.FileID, mapID uint16) error

	// UpdateStackDeltaPages defines a function that updates the mapping in a eBPF map from
	// a FileID and page to its stack delta lookup information.
	UpdateStackDeltaPages(fileID host.FileID, numDeltasPerPage []uint16,
		mapID uint16, firstPageAddr uint64) error

	// DeleteStackDeltaPage defines a function that removes the element specified by fileID and page
	// from the eBPF map.
	DeleteStackDeltaPage(fileID host.FileID, page uint64) error

	// UpdatePidPageMappingInfo defines a function that updates the eBPF map
	// pid_page_to_mapping_info with the given pidAndPage and fileIDAndOffset encoded values
	// as key/value pair.
	UpdatePidPageMappingInfo(pid libpf.PID, prefix lpm.Prefix, fileID, bias uint64) error

	// DeletePidPageMappingInfo removes the elements specified by prefixes from eBPF map
	// pid_page_to_mapping_info and returns the number of elements removed.
	DeletePidPageMappingInfo(pid libpf.PID, prefixes []lpm.Prefix) (int, error)

	// CollectMetrics returns gathered errors for changes to eBPF maps.
	CollectMetrics() []metrics.Metric

	// SupportsGenericBatchOperations returns true if the kernel supports eBPF batch operations
	// on hash and array maps.
	SupportsGenericBatchOperations() bool

	// SupportsLPMTrieBatchOperations returns true if the kernel supports eBPF batch operations
	// on LPM trie maps.
	SupportsLPMTrieBatchOperations() bool
}

func InterpreterOffsetKeyValue(ebpfProgIndex uint16, fileID host.FileID,
	offsetRanges []util.Range) (key uint64, value support.OffsetRange, err error) {
	rLen := len(offsetRanges)
	if rLen < 1 || rLen > 2 {
		return 0, support.OffsetRange{}, fmt.Errorf("invalid ranges %v", offsetRanges)
	}
	//  The keys of this map are executable-id-and-offset-into-text entries, and
	//  the offset_range associated with them gives the precise area in that page
	//  where the main interpreter loop is located. This is required to unwind
	//  nicely from native code into interpreted code.
	key = uint64(fileID)
	first := offsetRanges[0]
	value = support.OffsetRange{
		Lower_offset1: first.Start,
		Upper_offset1: first.End,
		Program_index: ebpfProgIndex,
	}
	if len(offsetRanges) == 2 {
		// Fields {lower,upper}_offset2 may be used to specify an optional second range
		// of an interpreter function. This may be useful if the interpreter function
		// consists of two non-contiguous memory ranges, which may happen due to Hot/Cold
		// split compiler optimization
		second := offsetRanges[1]
		value.Lower_offset2 = second.Start
		value.Upper_offset2 = second.End
	}
	return key, value, nil
}
