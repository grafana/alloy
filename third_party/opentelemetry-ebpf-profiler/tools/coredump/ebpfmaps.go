// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"math/bits"
	"unsafe"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	pmebpf "go.opentelemetry.io/ebpf-profiler/processmanager/ebpfapi"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/util"
)

/*
#define TESTING_COREDUMP
#include <stdlib.h>
#include "../../support/ebpf/types.h"
#include "../../support/ebpf/extmaps.h"
*/
import "C"

// ebpfMapsCoredump implements the Stack Delta Manager and the Interpreter Manager
// required EbpfMaps interfaces to access the core dump.
type ebpfMapsCoredump struct {
	ctx *ebpfContext
}

var _ interpreter.EbpfHandler = &ebpfMapsCoredump{}

func (emc *ebpfMapsCoredump) RemoveReportedPID(libpf.PID) {
}

func (emc *ebpfMapsCoredump) CollectMetrics() []metrics.Metric {
	return []metrics.Metric{}
}

func (emc *ebpfMapsCoredump) UpdateInterpreterOffsets(ebpfProgIndex uint16,
	fileID host.FileID, offsetRanges []util.Range) error {
	key, value, err := pmebpf.InterpreterOffsetKeyValue(ebpfProgIndex, fileID, offsetRanges)
	if err != nil {
		return err
	}
	emc.ctx.addMap(unsafe.Pointer(&C.interpreter_offsets), C.u64(key), pfunsafe.FromPointer(&value))
	return nil
}

func (emc *ebpfMapsCoredump) UpdateProcData(t libpf.InterpreterType, pid libpf.PID,
	ptr unsafe.Pointer) error {
	switch t {
	case libpf.Dotnet:
		emc.ctx.addMap(unsafe.Pointer(&C.dotnet_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_DotnetProcInfo))
	case libpf.Perl:
		emc.ctx.addMap(unsafe.Pointer(&C.perl_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_PerlProcInfo))
	case libpf.PHP:
		emc.ctx.addMap(unsafe.Pointer(&C.php_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_PHPProcInfo))
	case libpf.Python:
		emc.ctx.addMap(unsafe.Pointer(&C.py_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_PyProcInfo))
	case libpf.HotSpot:
		emc.ctx.addMap(unsafe.Pointer(&C.hotspot_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_HotspotProcInfo))
	case libpf.Ruby:
		emc.ctx.addMap(unsafe.Pointer(&C.ruby_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_RubyProcInfo))
	case libpf.V8:
		emc.ctx.addMap(unsafe.Pointer(&C.v8_procs), C.u32(pid), sliceBuffer(ptr, C.sizeof_V8ProcInfo))
	}
	return nil
}

func (emc *ebpfMapsCoredump) DeleteProcData(t libpf.InterpreterType, pid libpf.PID) error {
	switch t {
	case libpf.Dotnet:
		emc.ctx.delMap(unsafe.Pointer(&C.dotnet_procs), C.u32(pid))
	case libpf.Perl:
		emc.ctx.delMap(unsafe.Pointer(&C.perl_procs), C.u32(pid))
	case libpf.PHP:
		emc.ctx.delMap(unsafe.Pointer(&C.php_procs), C.u32(pid))
	case libpf.Python:
		emc.ctx.delMap(unsafe.Pointer(&C.py_procs), C.u32(pid))
	case libpf.HotSpot:
		emc.ctx.delMap(unsafe.Pointer(&C.hotspot_procs), C.u32(pid))
	case libpf.Ruby:
		emc.ctx.delMap(unsafe.Pointer(&C.ruby_procs), C.u32(pid))
	case libpf.V8:
		emc.ctx.delMap(unsafe.Pointer(&C.v8_procs), C.u32(pid))
	}
	return nil
}

func (emc *ebpfMapsCoredump) UpdatePidInterpreterMapping(pid libpf.PID,
	prefix lpm.Prefix, interpreterProgram uint8, fileID host.FileID, bias uint64) error {
	ctx := emc.ctx
	// pid_page_to_mapping_info is a LPM trie and expects the pid and page
	// to be in big endian format.
	bePid := bits.ReverseBytes32(uint32(pid))
	bePage := bits.ReverseBytes64(prefix.Key)

	biasAndUnwindProgram, err := support.EncodeBiasAndUnwindProgram(bias, interpreterProgram)
	if err != nil {
		return err
	}

	cKey := C.PIDPage{
		prefixLen: C.u32(support.BitWidthPID + prefix.Length),
		pid:       C.u32(bePid),
		page:      C.u64(bePage),
	}

	cValue := C.malloc(C.sizeof_PIDPageMappingInfo)
	*(*C.PIDPageMappingInfo)(cValue) = C.PIDPageMappingInfo{
		file_id:                 C.u64(fileID),
		bias_and_unwind_program: C.u64(biasAndUnwindProgram),
	}

	ctx.pidToPageMapping[cKey] = cValue
	return nil
}

func (emc *ebpfMapsCoredump) DeletePidInterpreterMapping(pid libpf.PID,
	prefix lpm.Prefix) error {
	ctx := emc.ctx
	// pid_page_to_mapping_info is a LPM trie and expects the pid and page
	// to be in big endian format.
	bePid := bits.ReverseBytes32(uint32(pid))
	bePage := bits.ReverseBytes64(prefix.Key)

	cKey := C.PIDPage{
		prefixLen: C.u32(support.BitWidthPID + prefix.Length),
		pid:       C.u32(bePid),
		page:      C.u64(bePage),
	}

	if value, ok := ctx.pidToPageMapping[cKey]; ok {
		C.free(value)
		delete(ctx.pidToPageMapping, cKey)
	}

	return nil
}

// Stack delta management
func (emc *ebpfMapsCoredump) UpdateUnwindInfo(index uint16, info sdtypes.UnwindInfo) error {
	if index >= support.UnwindInfoMaxEntries {
		return fmt.Errorf("unwind info array full (%d/%d items)",
			index, support.UnwindInfoMaxEntries)
	}

	cmd := (*C.UnwindInfo)(unsafe.Pointer(uintptr(emc.ctx.unwindInfoArray) +
		uintptr(index)*C.sizeof_UnwindInfo))
	*cmd = C.UnwindInfo{
		opcode:      C.u8(info.Opcode),
		fpOpcode:    C.u8(info.FPOpcode),
		mergeOpcode: C.u8(info.MergeOpcode),
		param:       C.s32(info.Param),
		fpParam:     C.s32(info.FPParam),
	}
	return nil
}

// Stack delta management
func (emc *ebpfMapsCoredump) UpdateExeIDToStackDeltas(fileID host.FileID,
	deltaArrays []pmebpf.StackDeltaEBPF) (uint16, error) {
	entSize := C.sizeof_StackDelta
	deltas := C.malloc(C.size_t(len(deltaArrays) * entSize))
	for index, delta := range deltaArrays {
		info := (*C.StackDelta)(unsafe.Pointer(uintptr(deltas) + uintptr(index*entSize)))
		*info = C.StackDelta{
			addrLow:    C.u16(delta.AddressLow),
			unwindInfo: C.u16(delta.UnwindInfo),
		}
	}
	ctx := emc.ctx
	// The coredump framework has only one map because we don't have the kernel limitation
	// of requiring fixed size inner maps. Return StackDeltaBucketSmallest as fixed mapID.
	ctx.exeIDToStackDeltaMaps[C.u64(fileID)] = deltas
	return support.StackDeltaBucketSmallest, nil
}

func (emc *ebpfMapsCoredump) DeleteExeIDToStackDeltas(fileID host.FileID,
	_ uint16) error {
	ctx := emc.ctx
	key := C.u64(fileID)
	if value, ok := ctx.exeIDToStackDeltaMaps[key]; ok {
		C.free(value)
		delete(ctx.exeIDToStackDeltaMaps, key)
	}
	return nil
}

func (emc *ebpfMapsCoredump) UpdateStackDeltaPages(fileID host.FileID, numDeltasPerPage []uint16,
	mapID uint16, firstPageAddr uint64,
) error {
	ctx := emc.ctx
	firstDelta := uint32(0)

	for pageNumber, numDeltas := range numDeltasPerPage {
		pageAddr := firstPageAddr + uint64(pageNumber)<<support.StackDeltaPageBits
		key := C.StackDeltaPageKey{
			fileID: C.u64(fileID),
			page:   C.u64(pageAddr),
		}
		value := C.malloc(C.sizeof_StackDeltaPageInfo)
		*(*C.StackDeltaPageInfo)(value) = C.StackDeltaPageInfo{
			firstDelta: C.u32(firstDelta),
			numDeltas:  C.u16(numDeltas),
			mapID:      C.u16(mapID),
		}
		firstDelta += uint32(numDeltas)
		ctx.stackDeltaPageToInfo[key] = value
	}

	return nil
}

func (emc *ebpfMapsCoredump) DeleteStackDeltaPage(fileID host.FileID, page uint64) error {
	ctx := emc.ctx
	key := C.StackDeltaPageKey{
		fileID: C.u64(fileID),
		page:   C.u64(page),
	}
	C.free(ctx.stackDeltaPageToInfo[key])
	delete(ctx.stackDeltaPageToInfo, key)
	return nil
}

func (emc *ebpfMapsCoredump) UpdatePidPageMappingInfo(pid libpf.PID, prefix lpm.Prefix,
	fileID, bias uint64) error {
	return emc.UpdatePidInterpreterMapping(pid, prefix, support.ProgUnwindNative,
		host.FileID(fileID), bias)
}

func (emc *ebpfMapsCoredump) DeletePidPageMappingInfo(pid libpf.PID, prefixes []lpm.Prefix) (int,
	error) {
	var deleted int
	for _, prefix := range prefixes {
		if err := emc.DeletePidInterpreterMapping(pid, prefix); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (emc *ebpfMapsCoredump) SupportsGenericBatchOperations() bool {
	return false
}

func (emc *ebpfMapsCoredump) SupportsLPMTrieBatchOperations() bool {
	return false
}
