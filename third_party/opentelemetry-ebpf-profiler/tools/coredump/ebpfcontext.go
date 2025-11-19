// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"unsafe"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/support"
)

/*
#define TESTING_COREDUMP
#include <stdlib.h>
#include "../../support/ebpf/types.h"
#include "../../support/ebpf/extmaps.h"
*/
import "C"

// ebpfContext is the context for EBPF code regarding the process it's unwinding.
type ebpfContext struct {
	// trace will contain the trace from the CGO executed eBPF unwinding code
	trace host.Trace

	// remotememory provides access to the target process memory space
	remoteMemory remotememory.RemoteMemory

	// PIDandTGID is the value for bpf_get_current_pid_tgid(), and is also the
	// unique context ID passed from eBPF code to the helper functions written
	// in Go to find the matching ebpfContext struct
	PIDandTGID C.u64

	// perCPURecord is the ebpf code PerCPURecord
	perCPURecord unsafe.Pointer

	// unwindInfoArray is the ebpf map unwind_info_array
	unwindInfoArray unsafe.Pointer

	// pidToPageMapping is the equivalent ebpf map implemented in Go. Special
	// support is needed to handle the prefix lookup.
	pidToPageMapping map[C.PIDPage]unsafe.Pointer

	// pidToPageMapping is the equivalent ebpf map implemented in Go. Special
	// support is needed to handle the key structure.
	stackDeltaPageToInfo map[C.StackDeltaPageKey]unsafe.Pointer

	// exeIDToStackDeltaMaps is the equivalent ebpf map implemented in Go.
	// Implemented separately to handle the nested map, and improve performance.
	exeIDToStackDeltaMaps map[C.u64]unsafe.Pointer

	// maps is the generic ebpf map implementation and implements all the
	// ebpf maps that do not need special handling (maps defined above)
	maps map[unsafe.Pointer]map[any]unsafe.Pointer

	// stackDeltaFileID is context variable for nested map lookups
	stackDeltaFileID C.u64
}

// ebpfContextMap is global mapping of EBPFContext id (PIDandTGID) to the actual data.
// This is needed to have the ebpf helpers written in Go to get access to the EBPFContext
// via given numeric ID (as Go pointers referring to memory with Go pointers cannot be
// passed directly to the C code).
var ebpfContextMap = map[C.u64]*ebpfContext{}

// newEBPFContext creates new EBPF Context from given core dump image
func newEBPFContext(pr process.Process) *ebpfContext {
	pid := pr.PID()
	ctx := &ebpfContext{
		trace:                 host.Trace{PID: pid},
		remoteMemory:          pr.GetRemoteMemory(),
		PIDandTGID:            C.u64(pid) << 32,
		pidToPageMapping:      make(map[C.PIDPage]unsafe.Pointer),
		stackDeltaPageToInfo:  make(map[C.StackDeltaPageKey]unsafe.Pointer),
		exeIDToStackDeltaMaps: make(map[C.u64]unsafe.Pointer),
		maps:                  make(map[unsafe.Pointer]map[any]unsafe.Pointer),
		perCPURecord:          C.malloc(C.sizeof_PerCPURecord),
		unwindInfoArray:       C.malloc(C.sizeof_UnwindInfo * C.ulong(support.UnwindInfoMaxEntries)),
	}
	ebpfContextMap[ctx.PIDandTGID] = ctx
	return ctx
}

func (ec *ebpfContext) addMap(mapPtr unsafe.Pointer, key any, value []byte) {
	innerMap, ok := ec.maps[mapPtr]
	if !ok {
		innerMap = make(map[any]unsafe.Pointer)
		ec.maps[mapPtr] = innerMap
	}
	innerMap[key] = C.CBytes(value)
}

func (ec *ebpfContext) delMap(mapPtr unsafe.Pointer, key any) {
	if innerMap, ok := ec.maps[mapPtr]; ok {
		if value, ok2 := innerMap[key]; ok2 {
			C.free(value)
			delete(innerMap, key)
		}
	}
}

func (ec *ebpfContext) resetTrace() {
	ec.trace.Frames = ec.trace.Frames[0:0]
}

func (ec *ebpfContext) release() {
	C.free(ec.perCPURecord)
	C.free(ec.unwindInfoArray)

	for pidPage, pageInfo := range ec.pidToPageMapping {
		C.free(pageInfo)
		delete(ec.pidToPageMapping, pidPage)
	}
	for deltaKey, deltaInfo := range ec.stackDeltaPageToInfo {
		C.free(deltaInfo)
		delete(ec.stackDeltaPageToInfo, deltaKey)
	}

	for fileID, stackDeltaMap := range ec.exeIDToStackDeltaMaps {
		C.free(stackDeltaMap)
		delete(ec.exeIDToStackDeltaMaps, fileID)
	}

	for mapName, innerMap := range ec.maps {
		for _, value := range innerMap {
			C.free(value)
		}
		delete(ec.maps, mapName)
	}

	delete(ebpfContextMap, ec.PIDandTGID)
}
