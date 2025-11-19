// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

// This file contains Go functions exported to the C ebpf code. This needs to be
// in separate file:
// Using //export in a file places a restriction on the preamble: since it is copied
// into two different C output files, it must not contain any definitions, only
// declarations. If a file contains both definitions and declarations, then the two
// output files will produce duplicate symbols and the linker will fail. To avoid
// this, definitions must be placed in preambles in other files, or in C source files.

import (
	"math/bits"
	"unsafe"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/times"

	log "github.com/sirupsen/logrus"
)

/*
#define TESTING_COREDUMP
#include "../../support/ebpf/types.h"
#include "../../support/ebpf/extmaps.h"
*/
import "C"

//export __bpf_log
func __bpf_log(buf unsafe.Pointer, sz C.int) {
	log.Info(string(sliceBuffer(buf, sz)))
}

//export __push_frame
func __push_frame(id, file, line C.u64, frameType, returnAddress C.uchar) C.int {
	ctx := ebpfContextMap[id]

	ctx.trace.Frames = append(ctx.trace.Frames, host.Frame{
		File:          host.FileID(file),
		Lineno:        libpf.AddressOrLineno(line),
		Type:          libpf.FrameType(frameType),
		ReturnAddress: returnAddress != 0,
	})

	return C.ERR_OK
}

//export bpf_ktime_get_ns
func bpf_ktime_get_ns() C.ulonglong {
	return C.ulonglong(times.GetKTime())
}

//export bpf_get_current_comm
func bpf_get_current_comm(buf unsafe.Pointer, sz C.int) C.int {
	copy(sliceBuffer(buf, sz), "comm")
	return 0
}

//export __bpf_probe_read_user
func __bpf_probe_read_user(id C.u64, buf unsafe.Pointer, sz C.int, ptr unsafe.Pointer) C.long {
	ctx := ebpfContextMap[id]
	dst := sliceBuffer(buf, sz)
	if _, err := ctx.remoteMemory.ReadAt(dst, int64(uintptr(ptr))); err != nil {
		return -1
	}
	return 0
}

// stackDeltaInnerMap is a special map returned to C code to indicate that
// we are accessing one of nested maps in the exe_id_to_X_stack_deltas maps
var stackDeltaInnerMap = C.malloc(1)

//export __bpf_map_lookup_elem
func __bpf_map_lookup_elem(id C.u64, mapdef unsafe.Pointer, keyptr unsafe.Pointer) unsafe.Pointer {
	ctx := ebpfContextMap[id]
	switch mapdef {
	case unsafe.Pointer(&C.pid_page_to_mapping_info):
		key := (*C.PIDPage)(keyptr)
		for key.prefixLen >= support.BitWidthPID {
			if val, ok := ctx.pidToPageMapping[*key]; ok {
				return val
			}
			key.prefixLen--
			shiftBits := support.BitWidthPID + support.BitWidthPage - key.prefixLen
			mask := uint64(0xffffffffffffffff) << shiftBits
			key.page &= C.ulonglong(bits.ReverseBytes64(mask))
		}
	case unsafe.Pointer(&C.per_cpu_records):
		return ctx.perCPURecord
	case unsafe.Pointer(&C.interpreter_offsets):
		if innerMap, ok := ctx.maps[mapdef]; ok {
			if val, ok := innerMap[*(*C.u64)(keyptr)]; ok {
				return val
			}
		}
	case unsafe.Pointer(&C.dotnet_procs), unsafe.Pointer(&C.perl_procs),
		unsafe.Pointer(&C.php_procs), unsafe.Pointer(&C.py_procs),
		unsafe.Pointer(&C.hotspot_procs), unsafe.Pointer(&C.ruby_procs),
		unsafe.Pointer(&C.v8_procs):
		if innerMap, ok := ctx.maps[mapdef]; ok {
			if val, ok := innerMap[*(*C.u32)(keyptr)]; ok {
				return val
			}
		}
	case unsafe.Pointer(&C.stack_delta_page_to_info):
		return ctx.stackDeltaPageToInfo[*(*C.StackDeltaPageKey)(keyptr)]
	case unsafe.Pointer(&C.exe_id_to_8_stack_deltas), unsafe.Pointer(&C.exe_id_to_9_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_10_stack_deltas), unsafe.Pointer(&C.exe_id_to_11_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_12_stack_deltas), unsafe.Pointer(&C.exe_id_to_13_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_14_stack_deltas), unsafe.Pointer(&C.exe_id_to_15_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_16_stack_deltas), unsafe.Pointer(&C.exe_id_to_17_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_18_stack_deltas), unsafe.Pointer(&C.exe_id_to_19_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_20_stack_deltas), unsafe.Pointer(&C.exe_id_to_21_stack_deltas),
		unsafe.Pointer(&C.exe_id_to_22_stack_deltas), unsafe.Pointer(&C.exe_id_to_23_stack_deltas):
		ctx.stackDeltaFileID = *(*C.u64)(keyptr)
		return stackDeltaInnerMap
	case unsafe.Pointer(&C.unwind_info_array):
		key := uintptr(*(*C.u32)(keyptr))
		return unsafe.Pointer(uintptr(ctx.unwindInfoArray) + key*C.sizeof_UnwindInfo)
	case stackDeltaInnerMap:
		key := uintptr(*(*C.u32)(keyptr))
		if deltas, ok := ctx.exeIDToStackDeltaMaps[ctx.stackDeltaFileID]; ok {
			return unsafe.Pointer(uintptr(deltas) + key*C.sizeof_StackDelta)
		}
	case unsafe.Pointer(&C.metrics):
		return unsafe.Pointer(uintptr(0))
	default:
		log.Errorf("Map at 0x%x not found", mapdef)
	}
	return unsafe.Pointer(uintptr(0))
}
