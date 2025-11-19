// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	npsr "go.opentelemetry.io/ebpf-profiler/nopanicslicereader"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/successfailurecounter"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/util"
)

// dotnet internal constants which have not changed through the current
// git repository life time, and are unlikely to change.
const (
	// MethodDesc's method classification as defined in:
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/method.hpp#L93
	mcIL = iota
	mcFCall
	mcNDirect
	mcEEImpl
	mcArray
	mcInstantiated
	mcComInterop
	mcDynamic
	mdcClassificationMask = 7

	// enum RangeSectionFlags
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.h#L628
	rangeSectionCodeHeap = 2
	// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/codeman.h#L664
	rangeSectionRangelist = 4
)

var methodClassficationName = []string{
	mcIL:           "method",
	mcFCall:        "fcall",
	mcNDirect:      "ndirect",
	mcEEImpl:       "eeimpl",
	mcArray:        "array",
	mcInstantiated: "instantiated",
	mcComInterop:   "cominterop",
	mcDynamic:      "dynamic",
}

// non-JIT code / stub types
const (
	// according to dotnet8 enum StubCodeBlockKind
	// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/codeman.h#L97
	codeStubUnknown = iota
	codeStubJump
	codeStubPrecode
	codeStubDynamicHelper
	codeStubStubPrecode
	codeStubFixupPrecode
	codeStubVirtualCallDispatch
	codeStubVirtualCallResolve
	codeStubVirtualCallLookup
	codeStubVirtualCallVtable

	// additional entries from dotnet6 and dotnet7
	codeStubLink
	codeStubThunkHeap
	codeStubDelegateInvoke
	codeStubVirtualCallCacheEntry

	// synthetic entries
	codeDynamic
	codeReadyToRun

	// keep these in sync with the dotnet_tracer.c
	codeJIT      = 0x1f
	codeFlagLeaf = 0x80
)

var codeName = []string{
	codeStubJump:                "jump",
	codeStubPrecode:             "precode",
	codeStubDynamicHelper:       "dynamic helper",
	codeStubStubPrecode:         "stub precode",
	codeStubFixupPrecode:        "fixup precode",
	codeStubVirtualCallDispatch: "VC/dispatch",
	codeStubVirtualCallResolve:  "VC/resolve",
	codeStubVirtualCallLookup:   "VC/lookup",
	codeStubVirtualCallVtable:   "VC/vtable",

	codeStubLink:                  "link",
	codeStubThunkHeap:             "thunk",
	codeStubDelegateInvoke:        "delegate",
	codeStubVirtualCallCacheEntry: "VC/cache",
	codeDynamic:                   "dynamic",
}

// dotnetMapping reflects mapping of PE file to process.
type dotnetMapping struct {
	start, end uint64
	info       *peInfo
}

type dotnetRangeSection struct {
	prefixes []lpm.Prefix
}

type dotnetInstance struct {
	interpreter.InstanceStubs

	// Symbolization metrics
	successCount atomic.Uint64
	failCount    atomic.Uint64

	// d is the interpreter data from dotnet (shared between processes)
	d *dotnetData
	// rm is used to access the remote process memory
	rm remotememory.RemoteMemory
	// bias is the ELF DSO load bias
	bias libpf.Address

	// Internal class instance pointers
	codeRangeListPtr                 libpf.Address
	precodeStubManagerPtr            libpf.Address
	stubLinkStubManagerPtr           libpf.Address
	thunkHeapStubManagerPtr          libpf.Address
	delegateInvokeStubManagerPtr     libpf.Address
	virtualCallStubManagerManagerPtr libpf.Address

	// mappings contains the PE mappings to process memory space. Multiple individual
	// consecutive process.Mappings may be merged to one mapping per PE file.
	mappings []dotnetMapping

	ranges map[libpf.Address]dotnetRangeSection

	rangeSectionSeen map[libpf.Address]libpf.Void

	// moduleToPEInfo maps Module* to it's peInfo. Since a dotnet instance will have
	// limited number of PE files mapped in, this is a map instead of a LRU.
	moduleToPEInfo map[libpf.Address]*peInfo

	addrToMethod *freelru.LRU[libpf.Address, *dotnetMethod]
}

// appendStubFrame appends a stub frame of given type
func (i *dotnetInstance) appendStubFrame(frames *libpf.Frames, codeType uint) {
	frames.Append(&libpf.Frame{
		Type:         libpf.DotnetFrame,
		FunctionName: libpf.Intern("[stub: " + codeName[codeType] + "]"),
	})
}

// addRange inserts a known memory mapping along with the needed data of it to ebpf maps
func (i *dotnetInstance) addRange(ebpf interpreter.EbpfHandler, pid libpf.PID,
	lowAddress, highAddress, mapBase libpf.Address, stubTypeOrHdrMap uint64) {
	// Inform the unwinder about this range
	prefixes, err := lpm.CalculatePrefixList(uint64(lowAddress), uint64(highAddress))
	if err != nil {
		log.Debugf("Failed to calculate lpm: %v", err)
		return
	}

	// Known stub types that have no stack frame
	switch stubTypeOrHdrMap {
	case codeStubPrecode, codeStubFixupPrecode, codeStubLink, codeStubThunkHeap,
		codeStubDelegateInvoke, codeStubVirtualCallVtable:
		stubTypeOrHdrMap |= codeFlagLeaf
	}

	rs := dotnetRangeSection{
		prefixes: prefixes,
	}
	i.ranges[lowAddress] = rs

	for _, prefix := range rs.prefixes {
		err := ebpf.UpdatePidInterpreterMapping(pid, prefix, support.ProgUnwindDotnet,
			host.FileID(stubTypeOrHdrMap), uint64(mapBase))
		if err != nil {
			log.Debugf("Failed to update interpreter mapping: %v", err)
		}
	}
}

// walkRangeList processes stub ranges from a RangeList
func (i *dotnetInstance) walkRangeList(ebpf interpreter.EbpfHandler, pid libpf.PID,
	headPtr libpf.Address, codeType uint) {
	// This hardcodes the layout of RangeList, Range and RangeListBlock from
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/utilcode.h#L3556-L3579
	const numRangesInBlock = 10
	const rangeSize = 3 * 8
	blockSize := uint(numRangesInBlock*rangeSize + 8)
	block := make([]byte, blockSize)

	flagLeaf := uint(0)
	stubName := codeName[codeType]
	log.Debugf("Found %s stub range list head at %x", stubName, headPtr)
	blockNum := 0
	for blockPtr := headPtr + 0x8; blockPtr != 0; blockNum++ {
		if err := i.rm.Read(blockPtr, block); err != nil {
			log.Debugf("Failed to read %s stub range block %d: %v",
				stubName, blockNum, err)
			return
		}
		for index := range uint(numRangesInBlock) {
			startAddr := npsr.Ptr(block, index*rangeSize)
			endAddr := npsr.Ptr(block, index*rangeSize+8)
			id := npsr.Ptr(block, index*rangeSize+16)
			if startAddr == 0 || endAddr == 0 || id == 0 {
				return
			}
			if _, ok := i.ranges[startAddr]; ok {
				continue
			}
			log.Debugf("pid %d: %s: %d/%d: rangeList %x-%x id %x",
				pid, stubName, blockNum, index, startAddr, endAddr, id)
			i.addRange(ebpf, pid, startAddr, endAddr, startAddr, uint64(codeType|flagLeaf))
		}
		blockPtr = npsr.Ptr(block, numRangesInBlock*rangeSize)
	}
}

// addRangeSection processes a RangeSection structure and calls addRange as needed
func (i *dotnetInstance) addRangeSection(ebpf interpreter.EbpfHandler, pid libpf.PID,
	rangeSection []byte) error {
	// Extract interesting fields
	vms := &i.d.vmStructs
	lowAddress := npsr.Ptr(rangeSection, vms.RangeSection.LowAddress)
	highAddress := npsr.Ptr(rangeSection, vms.RangeSection.HighAddress)
	flags := npsr.Uint32(rangeSection, vms.RangeSection.Flags)
	if _, ok := i.ranges[lowAddress]; ok {
		return nil
	}

	// Check for stub RangeList (dotnet8+)
	if vms.RangeSection.RangeList != 0 && flags&rangeSectionRangelist != 0 {
		rangeListPtr := npsr.Ptr(rangeSection, vms.RangeSection.RangeList)
		stubKind := i.rm.Uint32(rangeListPtr +
			libpf.Address(vms.CodeRangeMapRangeList.RangeListType))
		log.Debugf("%x-%x flags:%x  rangeListPtr %#x, type %d",
			lowAddress, highAddress, flags,
			rangeListPtr, stubKind)
		i.addRange(ebpf, pid, lowAddress, highAddress, lowAddress, uint64(stubKind))
	}

	// Determine and parse the heapListOrZapModule content
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/codeman.h#L640-L645
	if flags&rangeSectionCodeHeap != 0 {
		// heapListOrZapModule points to a heap list
		heapList := make([]byte, vms.HeapList.SizeOf)
		heapListPtr := npsr.Ptr(rangeSection, vms.RangeSection.HeapList)

		if err := i.rm.Read(heapListPtr, heapList); err != nil {
			log.Debugf("Failed to read heapList at %#x", heapListPtr)
			return err
		}
		mapBase := npsr.Ptr(heapList, vms.HeapList.MapBase)
		hdrMap := npsr.Ptr(heapList, vms.HeapList.HdrMap)
		heapListPtr = npsr.Ptr(heapList, vms.HeapList.Next)
		heapStart := npsr.Ptr(heapList, vms.HeapList.StartAddress)
		heapEnd := npsr.Ptr(heapList, vms.HeapList.EndAddress)

		log.Debugf("%x-%x flags:%x  heap: next:%x %x-%x mapBase: %x headerMap: %x",
			lowAddress, highAddress, flags,
			heapListPtr, heapStart, heapEnd, mapBase, hdrMap)

		i.addRange(ebpf, pid, lowAddress, highAddress, mapBase, uint64(hdrMap))
	} else {
		// heapListOrZapModule points to a Module.
		modulePtr := npsr.Ptr(rangeSection, vms.RangeSection.Module)
		// Find the memory mapping area for this module, and establish mapping from
		// Module* to the PE. This precaches the mapping for R2R modules and avoids
		// some remote memory reads.
		info, err := i.getPEInfoByAddress(uint64(lowAddress))
		if err != nil {
			return nil
		}
		i.moduleToPEInfo[modulePtr] = info
		log.Debugf("%x-%x flags:%x  module: %x -> %s",
			lowAddress, highAddress, flags,
			modulePtr, info.simpleName)
		i.addRange(ebpf, pid, lowAddress, highAddress, lowAddress, codeReadyToRun)
	}

	return nil
}

// walkRangeSectionList adds all RangeSections in a list (dotnet6 and dotnet7)
func (i *dotnetInstance) walkRangeSectionList(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	vms := &i.d.vmStructs
	rangeSection := make([]byte, vms.RangeSection.SizeOf)
	// walk the RangeSection list
	ptr := i.rm.Ptr(i.codeRangeListPtr)
	for ptr != 0 {
		if err := i.rm.Read(ptr, rangeSection); err != nil {
			return err
		}
		if err := i.addRangeSection(ebpf, pid, rangeSection); err != nil {
			return err
		}
		ptr = npsr.Ptr(rangeSection, vms.RangeSection.Next)
	}
	return nil
}

// walkRangeSectionMapFragments walks a RangeSectionMap::RangeSectionFragment list and processes
// the RangeSections from it.
func (i *dotnetInstance) walkRangeSectionMapFragments(ebpf interpreter.EbpfHandler, pid libpf.PID,
	fragmentPtr libpf.Address) error {
	// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/codeman.h#L974
	vms := &i.d.vmStructs
	fragment := make([]byte, 4*8)
	rangeSection := make([]byte, vms.RangeSection.SizeOf)
	for fragmentPtr != 0 {
		if err := i.rm.Read(fragmentPtr, fragment); err != nil {
			return fmt.Errorf("failed to read fragment: %v", err)
		}
		// Remove collectible bit
		fragmentPtr = npsr.Ptr(fragment, 0) &^ 1
		rangeSectionPtr := npsr.Ptr(fragment, 24)
		if _, ok := i.rangeSectionSeen[rangeSectionPtr]; ok {
			continue
		}
		i.rangeSectionSeen[rangeSectionPtr] = libpf.Void{}

		if err := i.rm.Read(rangeSectionPtr, rangeSection); err != nil {
			return fmt.Errorf("failed to read range section: %v", err)
		}
		if err := i.addRangeSection(ebpf, pid, rangeSection); err != nil {
			return err
		}
	}
	return nil
}

// walkRangeSectionMapLevel walks recursively a level index of a RangeSectionMap.
func (i *dotnetInstance) walkRangeSectionMapLevel(ebpf interpreter.EbpfHandler, pid libpf.PID,
	levelMapPtr libpf.Address, level uint) error {
	// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/codeman.h#L999-L1002
	const maxLevel = 5
	const entriesInLevel = 256
	levelPointers := make([]byte, entriesInLevel*8)

	if err := i.rm.Read(levelMapPtr, levelPointers); err != nil {
		return fmt.Errorf("failed to read section level: %v", err)
	}
	for index := uint(0); index < uint(len(levelPointers)); index += 8 {
		// mask out collectible bit
		ptr := npsr.Ptr(levelPointers, index) &^ 1
		if ptr == 0 {
			continue
		}
		if level < maxLevel {
			if err := i.walkRangeSectionMapLevel(ebpf, pid, ptr, level+1); err != nil {
				return err
			}
		} else {
			if err := i.walkRangeSectionMapFragments(ebpf, pid, ptr); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkRangeSectionMap processes a dotnet8 RangeSectionMap to enumerate all RangeSections
func (i *dotnetInstance) walkRangeSectionMap(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	i.rangeSectionSeen = make(map[libpf.Address]libpf.Void)
	err := i.walkRangeSectionMapLevel(ebpf, pid, i.codeRangeListPtr, 1)
	i.rangeSectionSeen = nil
	return err
}

func (i *dotnetInstance) getPEInfoByAddress(addressInModule uint64) (*peInfo, error) {
	idx, ok := slices.BinarySearchFunc(i.mappings, addressInModule,
		func(m dotnetMapping, addr uint64) int {
			if addr < m.start {
				return 1
			}
			if addr >= m.end {
				return -1
			}
			return 0
		})
	if !ok {
		return nil, fmt.Errorf("failed to find mapping for address %x", addressInModule)
	}

	mapping := &i.mappings[idx]
	return mapping.info, nil
}

func (i *dotnetInstance) getPEInfoByModulePtr(modulePtr libpf.Address) (*peInfo, error) {
	if info, ok := i.moduleToPEInfo[modulePtr]; ok {
		return info, nil
	}

	// If the Module does not have R2R executable code and we have not seen it yet,
	// we fallback to finding the PE info. The strategy is to read the SimpleName
	// member which is a pointer inside the memory mapped location of the PE .dll.
	// Read that and locate the memory mapping to get the PE info.
	vms := &i.d.vmStructs
	simpleNamePtr := i.rm.Ptr(modulePtr + libpf.Address(vms.Module.SimpleName))
	if simpleNamePtr == 0 {
		return nil, fmt.Errorf("module at %x, does not have name", modulePtr)
	}

	info, err := i.getPEInfoByAddress(uint64(simpleNamePtr))
	if err != nil {
		return nil, err
	}
	i.moduleToPEInfo[modulePtr] = info
	return info, nil
}

func (i *dotnetInstance) readMethod(methodDescPtr libpf.Address,
	debugInfoPtr libpf.Address) (*dotnetMethod, error) {
	vms := &i.d.vmStructs

	// Extract MethodDesc data
	methodDesc := make([]byte, vms.MethodDesc.SizeOf)
	if err := i.rm.Read(methodDescPtr, methodDesc); err != nil {
		return nil, err
	}

	tokenRemainder := npsr.Uint16(methodDesc, vms.MethodDesc.Flags3AndTokenRemainder)
	tokenRemainder &= vms.MethodDesc.TokenRemainderMask
	chunkIndex := npsr.Uint8(methodDesc, vms.MethodDesc.ChunkIndex)
	classification := npsr.Uint16(methodDesc, vms.MethodDesc.Flags) & mdcClassificationMask

	// Calculate the offset to the owning MethodDescChunk structure
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/method.hpp#L2321-L2328
	methodDescChunkPtr := methodDescPtr -
		libpf.Address(chunkIndex)*libpf.Address(vms.MethodDesc.Alignment) -
		libpf.Address(vms.MethodDescChunk.SizeOf)

	log.Debugf("method @%x: classification '%v', tokenRemainder %x, chunkIndex %x -> chunkPtr %x",
		methodClassficationName[classification], methodDescPtr,
		tokenRemainder, chunkIndex, methodDescChunkPtr)

	// Read the MethodDescChunk
	methodDescChunk := make([]byte, vms.MethodDescChunk.SizeOf)
	if err := i.rm.Read(methodDescChunkPtr, methodDescChunk); err != nil {
		return nil, err
	}
	methodTablePtr := npsr.Ptr(methodDescChunk, vms.MethodDescChunk.MethodTable)
	tokenRange := npsr.Uint16(methodDescChunk, vms.MethodDescChunk.TokenRange)
	tokenRange &= vms.MethodDescChunk.TokenRangeMask

	// Merge the MethodDesc and MethodDescChunk bits of Token value
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/method.hpp#L76-L80
	index := uint32(tokenRange)<<vms.MethodDesc.TokenRemainderBits + uint32(tokenRemainder)
	log.Debugf("methodchunk @%x: methodTablePtr %x: tokenRange %d, tokenRemainder %d -> index %d",
		methodDescChunkPtr, methodTablePtr, tokenRange, tokenRemainder, index)

	// Extract the loader module from the associated MethodTable
	// https://github.com/dotnet/runtime/blob/release/8.0/src/coreclr/vm/methodtable.cpp#L369-L383
	// FIXME: The dotnet runtime handles generic and array method differently.
	// Investigate if this needs adjustments to create correct method indexes.
	loaderModulePtr := i.rm.Ptr(methodTablePtr + libpf.Address(vms.MethodTable.LoaderModule))
	module, err := i.getPEInfoByModulePtr(loaderModulePtr)
	if err != nil {
		return nil, err
	}

	method := &dotnetMethod{
		classification: classification,
		index:          index,
		module:         module,
	}
	if debugInfoPtr != 0 {
		if err := method.readDebugInfo(newCachingReader(i.rm, int64(debugInfoPtr),
			1024), i.d); err != nil {
			log.Debugf("debug info reading failed: %v", err)
		}
	}
	return method, nil
}

func (i *dotnetInstance) getMethod(codeHeaderPtr libpf.Address) (*dotnetMethod, error) {
	if method, ok := i.addrToMethod.Get(codeHeaderPtr); ok {
		return method, nil
	}

	vms := &i.d.vmStructs
	codeHeader := make([]byte, vms.CodeHeader.SizeOf)
	if err := i.rm.Read(codeHeaderPtr, codeHeader); err != nil {
		return nil, err
	}

	debugInfoPtr := npsr.Ptr(codeHeader, vms.CodeHeader.DebugInfo)
	methodDescPtr := npsr.Ptr(codeHeader, vms.CodeHeader.MethodDesc)
	method, err := i.readMethod(methodDescPtr, debugInfoPtr)
	if err != nil {
		return nil, err
	}

	i.addrToMethod.Add(codeHeaderPtr, method)
	return method, nil
}

func (i *dotnetInstance) Detach(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	return ebpf.DeleteProcData(libpf.Dotnet, pid)
}

func (i *dotnetInstance) getDacSlot(slot uint) libpf.Address {
	// dotnet6 records everything as RVAs and starting dotnet7 they are pointers
	// see: https://github.com/dotnet/runtime/pull/68065
	dacTable := i.bias + libpf.Address(i.d.dacTableAddr)
	if i.d.version>>24 == 6 {
		slotPtr := dacTable + libpf.Address(slot*4)
		if rva := i.rm.Uint32(slotPtr); rva != 0 {
			return i.bias + libpf.Address(rva)
		}
	} else {
		slotPtr := dacTable + libpf.Address(slot*8)
		if ptr := i.rm.Ptr(slotPtr); ptr != 0 {
			return ptr
		}
	}
	return 0
}

func (i *dotnetInstance) getDacSlotPtr(slot uint) libpf.Address {
	ptr := i.getDacSlot(slot)
	if ptr == 0 {
		return 0
	}
	return i.rm.Ptr(ptr)
}

func (i *dotnetInstance) SynchronizeMappings(ebpf interpreter.EbpfHandler,
	exeReporter reporter.ExecutableReporter, pr process.Process,
	mappings []process.Mapping) error {
	// find pointer to codeRangeList if needed
	vms := &i.d.vmStructs
	if i.codeRangeListPtr == 0 {
		i.codeRangeListPtr = i.getDacSlot(vms.DacTable.ExecutionManagerCodeRangeList)
		if i.codeRangeListPtr == 0 {
			// This is normal state if we attached to the process before
			// the dotnet runtime has initialized itself fully.
			log.Debugf("Dotnet DAC table is not yet initialized at %x", i.d.dacTableAddr)
			return nil
		}
		log.Debugf("Found code range list head at %x", i.codeRangeListPtr)
	}
	if i.precodeStubManagerPtr == 0 && vms.DacTable.PrecodeStubManager != 0 {
		i.precodeStubManagerPtr = i.getDacSlotPtr(vms.DacTable.PrecodeStubManager)
	}
	if i.stubLinkStubManagerPtr == 0 && vms.DacTable.StubLinkStubManager != 0 {
		i.stubLinkStubManagerPtr = i.getDacSlotPtr(vms.DacTable.StubLinkStubManager)
	}
	if i.thunkHeapStubManagerPtr == 0 && vms.DacTable.ThunkHeapStubManager != 0 {
		i.thunkHeapStubManagerPtr = i.getDacSlotPtr(vms.DacTable.ThunkHeapStubManager)
	}
	if i.delegateInvokeStubManagerPtr == 0 && vms.DacTable.DelegateInvokeStubManager != 0 {
		i.delegateInvokeStubManagerPtr = i.getDacSlotPtr(vms.DacTable.DelegateInvokeStubManager)
	}
	if i.virtualCallStubManagerManagerPtr == 0 && vms.DacTable.VirtualCallStubManagerManager != 0 {
		i.virtualCallStubManagerManagerPtr = i.getDacSlotPtr(
			vms.DacTable.VirtualCallStubManagerManager)
	}

	// Collect PE files
	dotnetMappings := []dotnetMapping{}
	var prevKey util.OnDiskFileIdentifier
	var prevMaxVA uint64
	for idx := range mappings {
		m := &mappings[idx]
		// Some dotnet .dll files do not get executable mappings at all
		if m.IsAnonymous() {
			continue
		}
		if !strings.HasSuffix(m.Path.String(), ".dll") {
			continue
		}

		// Does this extend the previous mapping
		if prevKey == m.GetOnDiskFileIdentifier() && m.Vaddr < prevMaxVA {
			dotnetMappings[len(dotnetMappings)-1].end = m.Vaddr + m.Length
			continue
		}
		prevKey = m.GetOnDiskFileIdentifier()

		// Inspect the PE
		info := globalPeCache.Get(pr, m)
		if info.err != nil {
			return info.err
		}

		log.Debugf("%v -> %v guid %v", m.Path, info.simpleName, info.guid)

		exeReporter.ReportExecutable(&reporter.ExecutableMetadata{
			MappingFile: info.file,
			Process:     pr,
			Mapping:     m,
		})

		dotnetMappings = append(dotnetMappings, dotnetMapping{
			start: m.Vaddr,
			end:   m.Vaddr + m.Length,
			info:  info,
		})
		prevMaxVA = m.Vaddr + uint64(info.sizeOfImage)
	}

	// mappings are in sorted order
	i.mappings = dotnetMappings

	for _, m := range dotnetMappings {
		log.Debugf("mapped %x-%x %s", m.start, m.end, m.info.simpleName)
	}

	if err := i.d.walkRangeSectionsMethod(i, ebpf, pr.PID()); err != nil {
		log.Infof("Failed to walk code ranges: %v", err)
	}
	if i.precodeStubManagerPtr != 0 {
		if vms.PrecodeStubManager.StubPrecodeRangeList != 0 {
			rangeListPtr := i.precodeStubManagerPtr +
				libpf.Address(vms.PrecodeStubManager.StubPrecodeRangeList)
			i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubPrecode)
		}
		if vms.PrecodeStubManager.FixupPrecodeRangeList != 0 {
			rangeListPtr := i.precodeStubManagerPtr +
				libpf.Address(vms.PrecodeStubManager.FixupPrecodeRangeList)
			i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubFixupPrecode)
		}
	}
	if i.stubLinkStubManagerPtr != 0 {
		rangeListPtr := i.stubLinkStubManagerPtr + libpf.Address(vms.StubManager.SizeOf)
		i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubLink)
	}
	if i.thunkHeapStubManagerPtr != 0 {
		rangeListPtr := i.thunkHeapStubManagerPtr + libpf.Address(vms.StubManager.SizeOf)
		i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubThunkHeap)
	}
	if i.delegateInvokeStubManagerPtr != 0 {
		rangeListPtr := i.delegateInvokeStubManagerPtr + libpf.Address(vms.StubManager.SizeOf)
		i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubDelegateInvoke)
	}
	if i.virtualCallStubManagerManagerPtr != 0 && vms.VirtualCallStubManager.Next != 0 {
		managerPtr := i.virtualCallStubManagerManagerPtr + libpf.Address(vms.StubManager.SizeOf)
		managerPtr = i.rm.Ptr(managerPtr)
		for num := 0; managerPtr != 0 && num < 10; num++ {
			rangeListPtr := managerPtr + libpf.Address(vms.StubManager.SizeOf)

			// This hard codes the virtual call range list member order at:
			// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/virtualcallstub.h#L437
			// https://github.com/dotnet/runtime/blob/v8.0.4/src/coreclr/vm/virtualcallstub.h#L338
			switch i.d.version >> 24 {
			case 7:
				i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubVirtualCallLookup)

				rangeListPtr += libpf.Address(vms.LockedRangeList.SizeOf)
				i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubVirtualCallResolve)

				rangeListPtr += libpf.Address(vms.LockedRangeList.SizeOf)
				i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubVirtualCallDispatch)

				rangeListPtr += libpf.Address(vms.LockedRangeList.SizeOf)
				i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubVirtualCallCacheEntry)

				rangeListPtr += libpf.Address(vms.LockedRangeList.SizeOf)
				i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubVirtualCallVtable)
			case 8:
				i.walkRangeList(ebpf, pr.PID(), rangeListPtr, codeStubVirtualCallCacheEntry)
			}

			managerPtr = i.rm.Ptr(managerPtr + libpf.Address(vms.VirtualCallStubManager.Next))
		}
	}
	return nil
}

func (i *dotnetInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	addrToMethodStats := i.addrToMethod.ResetMetrics()

	return []metrics.Metric{
		{
			ID:    metrics.IDDotnetSymbolizationSuccesses,
			Value: metrics.MetricValue(i.successCount.Swap(0)),
		},
		{
			ID:    metrics.IDDotnetSymbolizationFailures,
			Value: metrics.MetricValue(i.failCount.Swap(0)),
		},
		{
			ID:    metrics.IDDotnetAddrToMethodHit,
			Value: metrics.MetricValue(addrToMethodStats.Hits),
		},
		{
			ID:    metrics.IDDotnetAddrToMethodMiss,
			Value: metrics.MetricValue(addrToMethodStats.Misses),
		},
	}, nil
}

func (i *dotnetInstance) Symbolize(frame *host.Frame, frames *libpf.Frames) error {
	if !frame.Type.IsInterpType(libpf.Dotnet) {
		return interpreter.ErrMismatchInterpreterType
	}

	sfCounter := successfailurecounter.New(&i.successCount, &i.failCount)
	defer sfCounter.DefaultToFailure()

	codeHeaderAndType := frame.File
	frameType := uint(codeHeaderAndType & 0x1f)
	codeHeaderPtr := libpf.Address(codeHeaderAndType >> 5)
	pcOffset := uint32(frame.Lineno)

	switch frameType {
	case codeReadyToRun:
		// Ready to Run (Non-JIT) frame running directly code from a PE file
		module, err := i.getPEInfoByAddress(uint64(codeHeaderPtr))
		if err != nil {
			return err
		}
		// The Line ID is the Relative Virtual Address (RVA) within the PE file where
		// PC is executing:
		// - on non-leaf frames, it is the return address
		// - on leaf frames, it is the address after the CALL machine opcode
		frames.Append(&libpf.Frame{
			Type:            libpf.DotnetFrame,
			AddressOrLineno: libpf.AddressOrLineno(pcOffset),
			FunctionName:    module.resolveR2RMethodName(pcOffset),
			SourceFile:      module.simpleName,
			MappingFile:     module.file,
		})
	case codeJIT:
		// JITted frame in anonymous mapping
		method, err := i.getMethod(codeHeaderPtr)
		if err != nil {
			return err
		}
		if method.index == 0 || method.classification == mcDynamic {
			i.appendStubFrame(frames, codeDynamic)
			break
		}

		ilOffset := method.mapPCOffsetToILOffset(pcOffset, frame.ReturnAddress)

		// The Line ID format is:
		//  4 bits  Set to 0xf to indicate JIT frame.
		// 28 bits  Method index
		// 32 bits  IL offset within that method. On non-leaf frames, it is
		//          pointing to CALL instruction if the debug info was accurate.
		lineID := libpf.AddressOrLineno(0xf0000000+method.index)<<32 +
			libpf.AddressOrLineno(ilOffset)
		methodName := method.module.resolveMethodName(method.index)
		frames.Append(&libpf.Frame{
			Type:            libpf.DotnetFrame,
			AddressOrLineno: lineID,
			SourceFile:      method.module.simpleName,
			FunctionName:    methodName,
			FunctionOffset:  ilOffset,
			MappingFile:     method.module.file,
		})
	default:
		// Stub code
		i.appendStubFrame(frames, frameType)
	}

	sfCounter.ReportSuccess()
	return nil
}
