// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"fmt"
	"unsafe"

	log "github.com/sirupsen/logrus"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/support"
)

type dotnetData struct {
	// version contains the version
	version uint32

	// dacTableAddr contains the ELF symbol 'g_dacTable' value
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/debug/ee/dactable.cpp#L80-L81
	dacTableAddr libpf.SymbolValue

	// method to walk range sections
	walkRangeSectionsMethod func(i *dotnetInstance, ebpf interpreter.EbpfHandler,
		pid libpf.PID) error

	vmStructs struct {
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/debug/ee/dactable.cpp#L81
		DacTable struct {
			// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/dacvars.h#L78
			ExecutionManagerCodeRangeList uint
			PrecodeStubManager            uint
			StubLinkStubManager           uint
			ThunkHeapStubManager          uint
			DelegateInvokeStubManager     uint
			VirtualCallStubManagerManager uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/lockedrangelist.h#L12
		LockedRangeList struct {
			SizeOf uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.h#L612
		RangeSection struct {
			LowAddress  uint
			HighAddress uint
			Next        uint
			Flags       uint
			HeapList    uint
			Module      uint
			RangeList   uint
			SizeOf      uint
		}
		// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/loaderallocator.hpp#L44
		CodeRangeMapRangeList struct {
			// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/loaderallocator.hpp#L180
			RangeListType uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.h#L466
		HeapList struct {
			Next         uint
			StartAddress uint
			EndAddress   uint
			MapBase      uint
			HdrMap       uint
			SizeOf       uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.h#L131-L135
		// NOTE: USE_INDIRECT_CODEHEADER is defined on architectures we care about, and this
		// really reflects the struct _hpRealCodeHdr.
		CodeHeader struct {
			DebugInfo  uint
			MethodDesc uint
			SizeOf     uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/method.hpp#L193
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/method.hpp#L1670
		MethodDesc struct {
			TokenRemainderMask uint16
			TokenRemainderBits uint
			Alignment          uint

			Flags3AndTokenRemainder uint
			ChunkIndex              uint
			Flags                   uint
			SizeOf                  uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/method.hpp#L2163
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/method.hpp#L2344
		MethodDescChunk struct {
			TokenRangeMask uint16
			MethodTable    uint
			TokenRange     uint
			SizeOf         uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/methodtable.h#L518
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/methodtable.h#L3548
		MethodTable struct {
			LoaderModule uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/ceeload.h#L601
		Module struct {
			SimpleName uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/patchpointinfo.h#L176-L190
		PatchpointInfo struct {
			SizeOf         uint
			NumberOfLocals uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/stubmgr.h#L204
		StubManager struct {
			SizeOf uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/stubmgr.h#L402
		PrecodeStubManager struct {
			StubPrecodeRangeList  uint
			FixupPrecodeRangeList uint
		}
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/virtualcallstub.h#L721-L768
		VirtualCallStubManager struct {
			Next uint
		}
	}
}

func (d *dotnetData) String() string {
	ver := d.version
	return fmt.Sprintf("dotnet %d.%d.%d", (ver>>24)&0xff, (ver>>16)&0xff, ver&0xffff)
}

func (d *dotnetData) Attach(ebpf interpreter.EbpfHandler, pid libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (interpreter.Instance, error) {
	log.Debugf("Attach PID %d, bias %x", pid, bias)

	addrToMethod, err := freelru.New[libpf.Address, *dotnetMethod](interpreter.LruFunctionCacheSize,
		libpf.Address.Hash32)
	if err != nil {
		return nil, err
	}

	procInfo := support.DotnetProcInfo{
		Version: d.version,
	}
	if err = ebpf.UpdateProcData(libpf.Dotnet, pid, unsafe.Pointer(&procInfo)); err != nil {
		return nil, err
	}

	return &dotnetInstance{
		d:              d,
		rm:             rm,
		bias:           bias,
		ranges:         make(map[libpf.Address]dotnetRangeSection),
		moduleToPEInfo: make(map[libpf.Address]*peInfo),
		addrToMethod:   addrToMethod,
	}, nil
}

func (d *dotnetData) Unload(_ interpreter.EbpfHandler) {
}

func (d *dotnetData) loadIntrospectionData() {
	vms := &d.vmStructs

	// Slot numbers
	vms.DacTable.ExecutionManagerCodeRangeList = 0x0
	vms.DacTable.StubLinkStubManager = 0xa
	vms.DacTable.ThunkHeapStubManager = 0xb

	// Addresses
	vms.LockedRangeList.SizeOf = 0x120
	vms.RangeSection.LowAddress = 0x0
	vms.RangeSection.HighAddress = 0x8

	vms.HeapList.Next = 0x0
	vms.HeapList.StartAddress = 0x10
	vms.HeapList.EndAddress = 0x18
	vms.HeapList.MapBase = 0x20
	vms.HeapList.HdrMap = 0x28
	vms.HeapList.SizeOf = 0x30

	vms.CodeHeader.DebugInfo = 0x0
	vms.CodeHeader.MethodDesc = 0x18 // NOTE: 0x20 if FEATURE_GDBJIT
	vms.CodeHeader.SizeOf = 0x20

	// NOTE: MethodDesc layout is quite different if _DEBUG build
	vms.MethodDesc.Alignment = 0x8
	vms.MethodDesc.Flags3AndTokenRemainder = 0x0
	vms.MethodDesc.ChunkIndex = 0x2
	vms.MethodDesc.Flags = 0x6
	vms.MethodDesc.SizeOf = 0x8

	vms.MethodDescChunk.MethodTable = 0
	vms.MethodDescChunk.TokenRange = 0x12
	vms.MethodDescChunk.SizeOf = 0x18

	vms.MethodTable.LoaderModule = 0x18 // NOTE: 0x20 if _DEBUG build

	vms.StubManager.SizeOf = 0x10

	// Version specific introspection data
	switch d.version >> 24 {
	case 6:
		vms.DacTable.DelegateInvokeStubManager = 0xe
		vms.DacTable.VirtualCallStubManagerManager = 0xf
		vms.RangeSection.Next = 0x18
		vms.RangeSection.Flags = 0x28
		vms.RangeSection.HeapList = 0x30
		vms.RangeSection.Module = 0x30
		vms.RangeSection.SizeOf = 0x38
		vms.MethodDesc.TokenRemainderBits = 14
		vms.Module.SimpleName = 0x8
		vms.PatchpointInfo.SizeOf = 20
		vms.PatchpointInfo.NumberOfLocals = 0
		d.walkRangeSectionsMethod = (*dotnetInstance).walkRangeSectionList
	case 7:
		vms.DacTable.DelegateInvokeStubManager = 0xe
		vms.DacTable.VirtualCallStubManagerManager = 0xf
		vms.RangeSection.Next = 0x18
		vms.RangeSection.Flags = 0x28
		vms.RangeSection.HeapList = 0x30
		vms.RangeSection.Module = 0x30
		vms.RangeSection.SizeOf = 0x38
		vms.MethodDesc.TokenRemainderBits = 13
		// Module inherits from ModuleBase with quite a bit of data
		// see: https://github.com/dotnet/runtime/pull/71271
		vms.Module.SimpleName = 0x100
		// PatchpointInfo was adjusted in:
		// https://github.com/dotnet/runtime/pull/65196
		// https://github.com/dotnet/runtime/pull/61712
		vms.PatchpointInfo.SizeOf = 32
		vms.PatchpointInfo.NumberOfLocals = 8
		// Contains useful information in dotnet7 only
		vms.DacTable.PrecodeStubManager = 0x9
		// Only present in dotnet7
		vms.PrecodeStubManager.StubPrecodeRangeList = vms.StubManager.SizeOf
		vms.PrecodeStubManager.FixupPrecodeRangeList = vms.StubManager.SizeOf +
			vms.LockedRangeList.SizeOf
		vms.VirtualCallStubManager.Next = 0x6e8
		d.walkRangeSectionsMethod = (*dotnetInstance).walkRangeSectionList
	case 8:
		vms.DacTable.VirtualCallStubManagerManager = 0xe
		vms.RangeSection.Flags = 0x10
		vms.RangeSection.Module = 0x20
		vms.RangeSection.HeapList = 0x28
		vms.RangeSection.RangeList = 0x30
		vms.RangeSection.SizeOf = 0x38
		vms.CodeRangeMapRangeList.RangeListType = 0x120
		vms.MethodDesc.TokenRemainderBits = 12
		vms.Module.SimpleName = 0x108
		vms.PatchpointInfo.SizeOf = 32
		vms.PatchpointInfo.NumberOfLocals = 8
		vms.VirtualCallStubManager.Next = 0x268
		d.walkRangeSectionsMethod = (*dotnetInstance).walkRangeSectionMap
	}

	// Calculated masks
	vms.MethodDesc.TokenRemainderMask = (1 << vms.MethodDesc.TokenRemainderBits) - 1
	vms.MethodDescChunk.TokenRangeMask = (1 << (24 - vms.MethodDesc.TokenRemainderBits)) - 1
}
