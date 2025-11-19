// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"runtime"
	"sync/atomic"
	"unsafe"

	log "github.com/sirupsen/logrus"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
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

// heapRange contains info for an individual heap.
type heapRange struct {
	codeStart, codeEnd     libpf.Address
	segmapStart, segmapEnd libpf.Address
}

// heapInfo contains info about all HotSpot heaps.
type heapInfo struct {
	segmentShift uint32
	ranges       []heapRange
}

type jitArea struct {
	start, end libpf.Address
	codeStart  libpf.Address
	tsid       uint64
}

// hotspotInstance contains information about one running HotSpot instance (pid)
type hotspotInstance struct {
	interpreter.InstanceStubs

	// Hotspot symbolization metrics
	successCount atomic.Uint64
	failCount    atomic.Uint64

	// d is the interpreter data from jvm.so (shared between processes)
	d *hotspotData

	// rm is used to access the remote process memory
	rm remotememory.RemoteMemory

	// bias is the ELF DSO load bias
	bias libpf.Address

	// prefixes is list of LPM prefixes added to ebpf maps (to be cleaned up)
	prefixes libpf.Set[lpm.Prefix]

	// addrToSymbol maps a JVM class Symbol address to it's string value
	addrToSymbol *freelru.LRU[libpf.Address, libpf.String]

	// addrToMethod maps a JVM class Method to a hotspotMethod which caches
	// the needed data from it.
	addrToMethod *freelru.LRU[libpf.Address, *hotspotMethod]

	// addrToJitInfo maps a JVM class nmethod to a hotspotJITInfo which caches
	// the needed data from it.
	addrToJITInfo *freelru.LRU[libpf.Address, *hotspotJITInfo]

	// addrToStubName maps a stub address to its name identifier.
	addrToStubName *freelru.LRU[libpf.Address, libpf.String]

	// mainMappingsInserted stores whether the heap areas and proc data are already populated.
	mainMappingsInserted bool

	// heapAreas stores the top-level JIT areas based on the Java heaps.
	heapAreas []jitArea

	// stubs stores all known stub routine regions.
	stubs map[libpf.Address]StubRoutine
}

func (d *hotspotInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	addrToSymbolStats := d.addrToSymbol.ResetMetrics()
	addrToMethodStats := d.addrToMethod.ResetMetrics()
	addrToJITInfoStats := d.addrToJITInfo.ResetMetrics()
	addrToStubNameStats := d.addrToStubName.ResetMetrics()

	return []metrics.Metric{
		{
			ID:    metrics.IDHotspotSymbolizationSuccesses,
			Value: metrics.MetricValue(d.successCount.Swap(0)),
		},
		{
			ID:    metrics.IDHotspotSymbolizationFailures,
			Value: metrics.MetricValue(d.failCount.Swap(0)),
		},
		{
			ID:    metrics.IDHotspotAddrToSymbolHit,
			Value: metrics.MetricValue(addrToSymbolStats.Hits),
		},
		{
			ID:    metrics.IDHotspotAddrToSymbolMiss,
			Value: metrics.MetricValue(addrToSymbolStats.Misses),
		},
		{
			ID:    metrics.IDHotspotAddrToSymbolAdd,
			Value: metrics.MetricValue(addrToSymbolStats.Inserts),
		},
		{
			ID:    metrics.IDHotspotAddrToSymbolDel,
			Value: metrics.MetricValue(addrToSymbolStats.Removals),
		},
		{
			ID:    metrics.IDHotspotAddrToMethodHit,
			Value: metrics.MetricValue(addrToMethodStats.Hits),
		},
		{
			ID:    metrics.IDHotspotAddrToMethodMiss,
			Value: metrics.MetricValue(addrToMethodStats.Misses),
		},
		{
			ID:    metrics.IDHotspotAddrToMethodAdd,
			Value: metrics.MetricValue(addrToMethodStats.Inserts),
		},
		{
			ID:    metrics.IDHotspotAddrToMethodDel,
			Value: metrics.MetricValue(addrToMethodStats.Removals),
		},
		{
			ID:    metrics.IDHotspotAddrToJITInfoHit,
			Value: metrics.MetricValue(addrToJITInfoStats.Hits),
		},
		{
			ID:    metrics.IDHotspotAddrToJITInfoMiss,
			Value: metrics.MetricValue(addrToJITInfoStats.Misses),
		},
		{
			ID:    metrics.IDHotspotAddrToJITInfoAdd,
			Value: metrics.MetricValue(addrToJITInfoStats.Inserts),
		},
		{
			ID:    metrics.IDHotspotAddrToJITInfoDel,
			Value: metrics.MetricValue(addrToJITInfoStats.Removals),
		},
		{
			ID:    metrics.IDHotspotAddrToStubNameIDHit,
			Value: metrics.MetricValue(addrToStubNameStats.Hits),
		},
		{
			ID:    metrics.IDHotspotAddrToStubNameIDMiss,
			Value: metrics.MetricValue(addrToStubNameStats.Misses),
		},
		{
			ID:    metrics.IDHotspotAddrToStubNameIDAdd,
			Value: metrics.MetricValue(addrToStubNameStats.Inserts),
		},
		{
			ID:    metrics.IDHotspotAddrToStubNameIDDel,
			Value: metrics.MetricValue(addrToStubNameStats.Removals),
		},
	}, nil
}

// getSymbol extracts a class Symbol value from the given address in the target JVM process
func (d *hotspotInstance) getSymbol(addr libpf.Address) libpf.String {
	if value, ok := d.addrToSymbol.Get(addr); ok {
		return value
	}
	vms := d.d.Get().vmStructs

	// Read the symbol length and readahead bytes in attempt to avoid second
	// system call to read the target string. 128 is chosen arbitrarily as "hopefully
	// good enough"; this value can be increased if it turns out to be necessary.
	var buf [128]byte
	if d.rm.Read(addr, buf[:]) != nil {
		return libpf.NullString
	}
	symLen := npsr.Uint16(buf[:], vms.Symbol.Length)
	if symLen == 0 {
		return libpf.NullString
	}

	// Always allocate the string separately so it does not hold the backing
	// buffer that might be larger than needed
	tmp := make([]byte, symLen)
	copy(tmp, buf[vms.Symbol.Body:])
	if vms.Symbol.Body+uint(symLen) > uint(len(buf)) {
		prefixLen := uint(len(buf[vms.Symbol.Body:]))
		if d.rm.Read(addr+libpf.Address(vms.Symbol.Body+prefixLen), tmp[prefixLen:]) != nil {
			return libpf.NullString
		}
	}
	s := pfunsafe.ToString(tmp)
	if !util.IsValidString(s) {
		log.Debugf("Extracted Hotspot symbol is invalid at 0x%x '%v'", addr, tmp)
		return libpf.NullString
	}
	value := libpf.Intern(s)
	d.addrToSymbol.Add(addr, value)
	return value
}

// getPoolSymbol reads a class ConstantPool value from given index, and reads the
// symbol value it is referencing
func (d *hotspotInstance) getPoolSymbol(addr libpf.Address, ndx uint16) libpf.String {
	// Zero index is not valid
	if ndx == 0 {
		return libpf.NullString
	}

	vms := &d.d.Get().vmStructs
	offs := libpf.Address(vms.ConstantPool.Sizeof) + 8*libpf.Address(ndx)
	cpoolVal := d.rm.Ptr(addr + offs)
	// The lowest bit is reserved by JVM to indicate if the value has been
	// resolved or not. The values see should be always resolved.
	// Just ignore the bit as it's meaning has changed between JDK versions.
	return d.getSymbol(cpoolVal &^ 1)
}

// getStubName read the stub name from the code blob at given address and generates a ID.
func (d *hotspotInstance) getStubName(ripOrBci uint32, addr libpf.Address) libpf.String {
	if value, ok := d.addrToStubName.Get(addr); ok {
		return value
	}
	vms := &d.d.Get().vmStructs
	constStubNameAddr := d.rm.Ptr(addr + libpf.Address(vms.CodeBlob.Name))
	stubName := d.rm.String(constStubNameAddr)

	a := d.rm.Ptr(addr+libpf.Address(vms.CodeBlob.CodeBegin)) + libpf.Address(ripOrBci)
	for _, stub := range d.stubs {
		if stub.start <= a && stub.end > a {
			stubName = fmt.Sprintf("%s [%s]", stubName, stub.name)
			break
		}
	}
	name := libpf.Intern(stubName)
	d.addrToStubName.Add(addr, name)
	return name
}

// getMethod reads and returns the interesting data from "class Method" at given address
func (d *hotspotInstance) getMethod(addr libpf.Address, _ uint32) (*hotspotMethod, error) {
	if value, ok := d.addrToMethod.Get(addr); ok {
		return value, nil
	}
	vms := &d.d.Get().vmStructs
	constMethodAddr := d.rm.Ptr(addr + libpf.Address(vms.Method.ConstMethod))
	constMethod := make([]byte, vms.ConstMethod.Sizeof)
	if err := d.rm.Read(constMethodAddr, constMethod); err != nil {
		return nil, fmt.Errorf("invalid ConstMethod ptr: %v", err)
	}

	cpoolAddr := npsr.Ptr(constMethod, vms.ConstMethod.Constants)
	cpool := make([]byte, vms.ConstantPool.Sizeof)
	if err := d.rm.Read(cpoolAddr, cpool); err != nil {
		return nil, fmt.Errorf("invalid ConstantPool ptr: %v", err)
	}

	instanceKlassAddr := npsr.Ptr(cpool, vms.ConstantPool.PoolHolder)
	instanceKlass := make([]byte, vms.InstanceKlass.Sizeof)
	if err := d.rm.Read(instanceKlassAddr, instanceKlass); err != nil {
		return nil, fmt.Errorf("invalid PoolHolder ptr: %v", err)
	}

	var sourceFileName libpf.String
	switch {
	case vms.ConstantPool.SourceFileNameIndex != 0:
		// JDK15
		sourceFileName = d.getPoolSymbol(cpoolAddr,
			npsr.Uint16(cpool, vms.ConstantPool.SourceFileNameIndex))
	case vms.InstanceKlass.SourceFileNameIndex != 0:
		// JDK8-14
		sourceFileName = d.getPoolSymbol(cpoolAddr,
			npsr.Uint16(instanceKlass, vms.InstanceKlass.SourceFileNameIndex))
	default:
		// JDK7
		sourceFileName = d.getSymbol(
			npsr.Ptr(instanceKlass, vms.InstanceKlass.SourceFileName))
	}
	klassName := d.getSymbol(npsr.Ptr(instanceKlass, vms.Klass.Name)).String()
	methodName := d.getPoolSymbol(cpoolAddr, npsr.Uint16(constMethod,
		vms.ConstMethod.NameIndex))
	signature := d.getPoolSymbol(cpoolAddr, npsr.Uint16(constMethod,
		vms.ConstMethod.SignatureIndex))

	if sourceFileName == libpf.NullString {
		// Java and Scala can autogenerate lambdas which have no source
		// information available. The HotSpot VM backtraces displays
		// "Unknown Source" as the filename for these.
		sourceFileName = interpreter.UnknownSourceFile

		// Java 15 introduced "Hidden Classes" via JEP 371. These class names
		// contain pointers. Mask the pointers to reduce cardinality.
		klassName = hiddenClassRegex.ReplaceAllString(klassName, hiddenClassMask)
	}

	// Synthesize a FileID that is unique to this Class/Method that can be
	// used as "CodeObjectID" value in the trace as frames FileID.
	// Keep the sourcefileName there to start with, and add klass name, method
	// name, byte code and the JVM presentation of the source line table.
	h := fnv.New128a()
	_, _ = h.Write([]byte(sourceFileName.String()))
	_, _ = h.Write([]byte(klassName))
	_, _ = h.Write([]byte(methodName.String()))
	_, _ = h.Write([]byte(signature.String()))

	// Read the byte code for CodeObjectID
	bytecodeSize := npsr.Uint16(constMethod, vms.ConstMethod.CodeSize)
	byteCode := make([]byte, bytecodeSize)
	err := d.rm.Read(constMethodAddr+libpf.Address(vms.ConstMethod.Sizeof), byteCode)
	if err != nil {
		return nil, fmt.Errorf("invalid ByteCode ptr: %v", err)
	}

	_, _ = h.Write(byteCode)

	var lineTable []byte
	startLine := ^uint32(0)
	// NOTE: ConstMethod.Flags is either u16 or u32 depending on JVM version. Since we
	//       only care about flags in the first byte and only operate on little endian
	//       architectures we can get away with reading it as u8 either way.
	if npsr.Uint8(constMethod, vms.ConstMethod.Flags)&ConstMethod_has_linenumber_table != 0 {
		// The line number table size is not known ahead of time. It is delta compressed,
		// so read it once using buffered read to capture it fully. Get also the smallest
		// line number present as the function start line number - this is not perfect
		// as it's the first line for which code was generated. Usually one or few lines
		// after the actual function definition line. The Byte Code Index (BCI) is just
		// used for additional method ID hash input.
		var pcLineEntry [4]byte
		var curBci, curLine uint32
		err = nil
		r := newRecordingReader(d.rm, int64(constMethodAddr)+int64(vms.ConstMethod.Sizeof)+
			int64(bytecodeSize), 256)
		dec := d.d.newUnsigned5Decoder(r)
		for err == nil {
			if curLine > 0 && curLine < startLine {
				startLine = curLine
			}
			err = dec.decodeLineTableEntry(&curBci, &curLine)

			// The BCI and line numbers are read from the target memory in the custom
			// format, but the .class file LineNumberTable is big-endian encoded
			// {
			//   u2 start_pc, line_number;
			// } line_number_table[line_number_table_length]
			//
			// This hashes the line_number_table in .class file format, so if we
			// ever start indexing .class/.java files to match methods to real source
			// file IDs, we can produce the hash in the indexer without additional
			// transformations needed.
			binary.BigEndian.PutUint16(pcLineEntry[0:2], uint16(curBci))
			binary.BigEndian.PutUint16(pcLineEntry[2:4], uint16(curLine))
			_, _ = h.Write(pcLineEntry[:])
		}

		// If EOF encountered, the table was processed successfully.
		if err == io.EOF {
			lineTable = r.GetBuffer()
		}
	}
	if startLine == ^uint32(0) {
		startLine = 0
	}
	// Finalize CodeObjectID generation
	objectID, err := libpf.FileIDFromBytes(h.Sum(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create a code object ID: %v", err)
	}

	demangledName := demangleJavaMethod(klassName, methodName.String(), signature.String())
	sym := &hotspotMethod{
		sourceFileName: sourceFileName,
		objectID:       objectID,
		methodName:     libpf.Intern(demangledName),
		bytecodeSize:   bytecodeSize,
		lineTable:      lineTable,
		startLineNo:    uint16(startLine),
	}
	d.addrToMethod.Add(addr, sym)
	return sym, nil
}

// getJITInfo reads and returns the interesting data from "class nmethod" at given address
func (d *hotspotInstance) getJITInfo(addr libpf.Address, addrCheck uint32) (
	*hotspotJITInfo, error) {
	// Each JIT-ted function is contained in a "class nmethod" (derived from CodeBlob,
	// and CompiledMethod [JDK22 and earlier]).
	//
	// see: src/hotspot/share/code/compiledMethod.hpp
	//      src/hotspot/share/code/nmethod.hpp
	//
	// scopes_data is a list of descriptors that lists the method and
	//   it's Byte Code Index (BCI) activations for the scope
	// scopes_pcs is a look up table to map RIP to scope_data
	// metadata is the array that maps scope_data method indices to "class Method"

	const maxMetadataSize = 1024 * 1024

	if jit, ok := d.addrToJITInfo.Get(addr); ok {
		if jit.compileID == addrCheck {
			return jit, nil
		}
	}
	vmd := d.d.Get()
	vms := &vmd.vmStructs
	nmethod := make([]byte, vms.Nmethod.Sizeof)
	if err := d.rm.Read(addr, nmethod); err != nil {
		return nil, fmt.Errorf("invalid nmethod ptr: %v", err)
	}

	// Since the Java VM might decide recompile or free the JITted nmethods
	// we use the nmethod._compile_id (global running number to identify JIT
	// method) to uniquely identify that we are using the right data here
	// vs. when the pointer was captured by eBPF.
	compileID := npsr.Uint32(nmethod, vms.Nmethod.CompileID)
	if compileID != addrCheck {
		return nil, errors.New("JIT info evicted since eBPF snapshot")
	}

	method, err := d.getMethod(npsr.Ptr(nmethod, vms.Nmethod.Method), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get JIT Method: %v", err)
	}

	// Finally read the associated debug information for this method
	var jit *hotspotJITInfo
	if vmd.version < 0x17000000 {
		// JDK22 and earlier
		//
		// Layout of important bits in such 'class nmethod' pointer is:
		// [class CodeBlob fields]
		// [class CompiledMethod fields]
		// [class nmethod fields]
		// ...
		// [JIT_code]		@ this + CodeBlob._code_start
		// ...
		// [metadata]		@ this + nmethod._metadata_offset	\ these three
		// [scopes_data]	@ CompiledMethod._scopes_data_begin	| arrays we need
		// [scopes_pcs]		@ this + nmethod._scopes_pcs_offset	/ for inlining info
		// [dependencies]	@ this + nmethod._dependencies_offset
		// ...
		var scopesDataOff libpf.Address
		metadataOff := npsr.PtrDiff32(nmethod, vms.Nmethod.MetadataOffset)
		if vmd.nmethodUsesOffsets != 0 {
			scopesDataOff = npsr.PtrDiff32(nmethod, vms.Nmethod.ScopesDataOffset)
		} else {
			scopesDataOff = npsr.Ptr(nmethod, vms.Nmethod.ScopesDataOffset) - addr
		}
		scopesPcsOff := npsr.PtrDiff32(nmethod, vms.Nmethod.ScopesPcsOffset)
		depsOff := npsr.PtrDiff32(nmethod, vms.Nmethod.DependenciesOffset)

		if metadataOff > scopesDataOff || scopesDataOff > scopesPcsOff || scopesPcsOff > depsOff {
			return nil, fmt.Errorf("unexpected nmethod layout: %v <= %v <= %v <= %v",
				metadataOff, scopesDataOff, scopesPcsOff, depsOff)
		}
		metadataSize := depsOff - metadataOff
		if metadataSize >= maxMetadataSize {
			return nil, fmt.Errorf("unreasonably large metadata data region: %d bytes",
				metadataSize)
		}
		scopesData := make([]byte, metadataSize)
		if err := d.rm.Read(addr+metadataOff, scopesData); err != nil {
			return nil, fmt.Errorf("invalid nmethod metadata: %v", err)
		}

		// Buffer is read starting from metadataOff, so adjust accordingly
		scopesDataOff -= metadataOff
		scopesPcsOff -= metadataOff

		jit = &hotspotJITInfo{
			compileID:  compileID,
			method:     method,
			metadata:   scopesData[:scopesDataOff],
			scopesData: scopesData[scopesDataOff:scopesPcsOff],
			scopesPcs:  scopesData[scopesPcsOff:],
		}
	} else {
		// JDK23 and later
		//
		// Each JIT-ted function is contained in a "class nmethod" (derived from CodeBlob).
		//
		// Layout of important bits in such 'class nmethod' pointer is:
		// [class CodeBlob fields]
		// [class nmethod fields]
		//   address		_immutable_data
		// ...
		// [JIT_code]		@ this + CodeBlob._code_start
		// ...
		// [metadata]		@ this + CodeBlob._code_end + nmethod._metadata_offset (JDK -24)
		// [metadata]		@ CodeBlob._mutable_data + CodeBlob._relocation_size   (JDK 25+)
		//
		// [scopes_data]	@ _immutable_data + nmethod._scopes_data_begin	\ arrays we need
		// [scopes_pcs]		@ _immutable_data + nmethod._scopes_pcs_offset	/ for inlining info
		// [speculations]	@ _immutable_data + nmethod._speculations_offset
		// [end]		@ _immutable_Data + nmethod._immutable_data_size
		// ...
		// speculations presence depends on JDK build, and is not used. Instead the scopes
		// end is determined from immutable data size.

		var metadataPtr, metadataSize libpf.Address
		if vms.CodeBlob.MutableData != 0 {
			relocationSize := npsr.PtrDiff32(nmethod, vms.CodeBlob.RelocationSize)
			mutableDataSize := npsr.PtrDiff32(nmethod, vms.CodeBlob.MutableDataSize)
			metadataPtr = npsr.Ptr(nmethod, vms.CodeBlob.MutableData) + relocationSize
			metadataSize = mutableDataSize - relocationSize
		} else {
			metadataOff := npsr.PtrDiff32(nmethod, vms.CodeBlob.CodeEnd) +
				npsr.PtrDiff16(nmethod, vms.Nmethod.MetadataOffset)
			metadataPtr = addr + metadataOff
			// Actually the metadata only spans to `_jvmci_data_offset`, but that field isn't exposed
			// through VMstructs, and the codeblob size is the next boundary after that.
			metadataSize = npsr.PtrDiff32(nmethod, vms.CodeBlob.Size) - metadataOff
		}

		scopesPcsOff := npsr.PtrDiff32(nmethod, vms.Nmethod.ScopesPcsOffset)
		scopesDataOff := npsr.PtrDiff32(nmethod, vms.Nmethod.ScopesDataOffset)
		immutableDataPtr := npsr.Ptr(nmethod, vms.Nmethod.ImmutableData)
		immutableDataSize := npsr.Uint32(nmethod, vms.Nmethod.ImmutableDataSize)
		if immutableDataSize >= maxMetadataSize {
			return nil, fmt.Errorf("unreasonably large immutable data region: %d bytes",
				immutableDataSize)
		}
		if scopesPcsOff > scopesDataOff || scopesDataOff > libpf.Address(immutableDataSize) {
			return nil, fmt.Errorf("unexpected immutable data layout: %v, %v, %v",
				scopesDataOff, scopesPcsOff, immutableDataSize)
		}
		if metadataSize >= maxMetadataSize {
			return nil, fmt.Errorf("unreasonably large nmethod metadata: %v",
				metadataSize)
		}

		metadata := make([]byte, metadataSize)
		if err := d.rm.Read(metadataPtr, metadata); err != nil {
			return nil, fmt.Errorf("invalid nmethod metadata ptr: %v", err)
		}

		// Since the beginning of immutable data is not needed, adjust to not read it
		immutableDataPtr += scopesPcsOff
		immutableDataSize -= uint32(scopesPcsOff)
		scopesDataOff -= scopesPcsOff
		scopesPcsOff = 0

		immutableData := make([]byte, immutableDataSize)
		if err := d.rm.Read(immutableDataPtr, immutableData); err != nil {
			return nil, fmt.Errorf("invalid immutable_data ptr: %v", err)
		}

		jit = &hotspotJITInfo{
			compileID:  compileID,
			method:     method,
			metadata:   metadata,
			scopesPcs:  immutableData[scopesPcsOff:scopesDataOff],
			scopesData: immutableData[scopesDataOff:],
		}
	}

	d.addrToJITInfo.Add(addr, jit)
	return jit, nil
}

// Detach removes all information regarding a given process from the eBPF maps.
func (d *hotspotInstance) Detach(ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	var err error
	if d.mainMappingsInserted {
		err = ebpf.DeleteProcData(libpf.HotSpot, pid)
	}

	for prefix := range d.prefixes {
		if err2 := ebpf.DeletePidInterpreterMapping(pid, prefix); err2 != nil {
			err = errors.Join(err,
				fmt.Errorf("failed to remove page 0x%x/%d: %v",
					prefix.Key, prefix.Length, err2))
		}
	}

	if err != nil {
		return fmt.Errorf("failed to detach hotspotInstance from PID %d: %v",
			pid, err)
	}
	return nil
}

// gatherHeapInfo collects information about HotSpot heaps.
func (d *hotspotInstance) gatherHeapInfo(vmd *hotspotVMData) (*heapInfo, error) {
	info := &heapInfo{}

	// Determine the location of heap pointers
	var heapPtrAddr libpf.Address
	var numHeaps uint32

	vms := &vmd.vmStructs
	rm := d.rm
	if vms.CodeCache.Heap != 0 {
		// JDK -8: one fixed CodeHeap through fixed pointer
		numHeaps = 1
		heapPtrAddr = vms.CodeCache.Heap + d.bias
	} else {
		// JDK 9-: CodeHeap through _heaps array
		heaps := make([]byte, vms.GrowableArrayInt.Sizeof)
		if err := rm.Read(rm.Ptr(vms.CodeCache.Heaps+d.bias), heaps); err != nil {
			return nil, fmt.Errorf("fail to read heap array: %v", err)
		}
		// Read numHeaps
		numHeaps = npsr.Uint32(heaps, vms.GenericGrowableArray.Len)

		heapPtrAddr = npsr.Ptr(heaps, vms.GrowableArrayInt.Data)
		if numHeaps == 0 || heapPtrAddr == 0 {
			// The heaps are not yet initialized
			return nil, nil
		}
	}

	// Get and sanity check the number of heaps
	if numHeaps < 1 || numHeaps > 16 {
		return nil, fmt.Errorf("bad hotspot heap count (%v)", numHeaps)
	}

	// Extract the heap pointers
	heap := make([]byte, vms.CodeHeap.Sizeof)
	heapPtrs := make([]byte, 8*numHeaps)
	if err := rm.Read(heapPtrAddr, heapPtrs); err != nil {
		return nil, fmt.Errorf("fail to read heap array values: %v", err)
	}

	// Extract each heap structure individually
	for ndx := uint32(0); ndx < numHeaps; ndx++ {
		heapPtr := npsr.Ptr(heapPtrs, uint(ndx*8))
		if heapPtr == 0 {
			// JVM is not initialized yet. Retry later.
			return nil, nil
		}
		if err := rm.Read(heapPtr, heap); err != nil {
			return nil, fmt.Errorf("fail to read heap pointer %d: %v", ndx, err)
		}

		// The segment shift is same for all heaps. So record it for the process only.
		info.segmentShift = npsr.Uint32(heap, vms.CodeHeap.Log2SegmentSize)

		// The LowBoundary and HighBoundary describe the mapping that was reserved
		// with mmap(PROT_NONE). The actual mapping that is committed memory is in
		// VirtualSpace.{Low,High}. However, since we are just following pointers we
		// really care about the maximum values which do not change.
		rng := heapRange{
			codeStart:   npsr.Ptr(heap, vms.CodeHeap.Memory+vms.VirtualSpace.LowBoundary),
			codeEnd:     npsr.Ptr(heap, vms.CodeHeap.Memory+vms.VirtualSpace.HighBoundary),
			segmapStart: npsr.Ptr(heap, vms.CodeHeap.Segmap+vms.VirtualSpace.LowBoundary),
			segmapEnd:   npsr.Ptr(heap, vms.CodeHeap.Segmap+vms.VirtualSpace.HighBoundary),
		}

		// Hook the memory area for HotSpot unwinder
		if rng.codeStart == 0 || rng.codeEnd == 0 {
			return nil, nil
		}

		info.ranges = append(info.ranges, rng)
	}

	return info, nil
}

// addJitArea inserts an entry into the PID<->interpreter BPF map.
func (d *hotspotInstance) addJitArea(ebpf interpreter.EbpfHandler,
	pid libpf.PID, area jitArea) error {
	prefixes, err := lpm.CalculatePrefixList(uint64(area.start), uint64(area.end))
	if err != nil {
		return fmt.Errorf("LPM prefix calculation error for %x-%x", area.start, area.end)
	}

	for _, prefix := range prefixes {
		if _, exists := d.prefixes[prefix]; exists {
			continue
		}

		if err = ebpf.UpdatePidInterpreterMapping(pid, prefix,
			support.ProgUnwindHotspot, host.FileID(area.tsid),
			uint64(area.codeStart)); err != nil {
			return fmt.Errorf(
				"failed to insert LPM entry for pid %d, page 0x%x/%d: %v",
				pid, prefix.Key, prefix.Length, err)
		}

		d.prefixes[prefix] = libpf.Void{}
	}

	log.Debugf("HotSpot jitArea: pid: %d, code %x-%x tsid: %x (%d tries)",
		pid, area.start, area.end, area.tsid, len(prefixes))

	return nil
}

// populateMainMappings populates all important BPF map entries that are available
// immediately after interpreter startup (once VM structs becomes available). This
// allows the BPF code to start unwinding even if some more detailed information
// about e.g. stub routines is not yet available.
func (d *hotspotInstance) populateMainMappings(vmd *hotspotVMData,
	ebpf interpreter.EbpfHandler, pid libpf.PID) error {
	if d.mainMappingsInserted {
		// Already populated: nothing to do here.
		return nil
	}

	heap, err := d.gatherHeapInfo(vmd)
	if err != nil {
		return err
	}
	if heap == nil || len(heap.ranges) == 0 {
		return nil
	}

	// Construct and insert heap areas.
	for _, rng := range heap.ranges {
		tsid := (uint64(rng.segmapStart) & support.HSTSIDSegMapMask) << support.HSTSIDSegMapBit

		area := jitArea{
			start:     rng.codeStart,
			end:       rng.codeEnd,
			codeStart: rng.codeStart,
			tsid:      tsid,
		}

		if err = d.addJitArea(ebpf, pid, area); err != nil {
			return err
		}

		d.heapAreas = append(d.heapAreas, area)
	}

	// Set up the main eBPF info structure.
	vms := &vmd.vmStructs
	procInfo := support.HotspotProcInfo{
		Nmethod_deopt_offset:   uint16(vms.Nmethod.DeoptimizeOffset),
		Nmethod_compileid:      uint16(vms.Nmethod.CompileID),
		Nmethod_orig_pc_offset: uint16(vms.Nmethod.OrigPcOffset),
		Codeblob_name:          uint8(vms.CodeBlob.Name),
		Codeblob_codestart:     uint8(vms.CodeBlob.CodeBegin),
		Codeblob_codeend:       uint8(vms.CodeBlob.CodeEnd),
		Codeblob_framecomplete: uint8(vms.CodeBlob.FrameCompleteOffset),
		Codeblob_framesize:     uint8(vms.CodeBlob.FrameSize),
		Cmethod_size:           uint8(vms.ConstMethod.Sizeof),
		Heapblock_size:         uint8(vms.HeapBlock.Sizeof),
		Method_constmethod:     uint8(vms.Method.ConstMethod),
		Jvm_version:            uint8(vmd.version >> 24),
		Segment_shift:          uint8(heap.segmentShift),
		Nmethod_uses_offsets:   vmd.nmethodUsesOffsets,
	}

	if vms.CodeCache.LowBound == 0 {
		// JDK-8 has only one heap, use its bounds
		procInfo.Codecache_start = uint64(heap.ranges[0].codeStart)
		procInfo.Codecache_end = uint64(heap.ranges[0].codeEnd)
	} else {
		// JDK9+ the VM tracks it separately
		procInfo.Codecache_start = uint64(d.rm.Ptr(vms.CodeCache.LowBound + d.bias))
		procInfo.Codecache_end = uint64(d.rm.Ptr(vms.CodeCache.HighBound + d.bias))
	}

	if err = ebpf.UpdateProcData(libpf.HotSpot, pid, unsafe.Pointer(&procInfo)); err != nil {
		return err
	}

	d.mainMappingsInserted = true
	return nil
}

// updateStubMappings adds new stub routines that are not yet tracked in our
// stubs map and, if necessary on the architecture, inserts unwinding instructions
// for them in the PID mappings BPF map.
func (d *hotspotInstance) updateStubMappings(vmd *hotspotVMData,
	ebpf interpreter.EbpfHandler, pid libpf.PID) {
	for _, stub := range findStubBounds(vmd, d.bias, d.rm) {
		if _, exists := d.stubs[stub.start]; exists {
			continue
		}

		d.stubs[stub.start] = stub

		// Separate stub areas are only required on ARM64.
		if runtime.GOARCH != "arm64" {
			continue
		}

		// Find corresponding heap jitArea.
		var stubHeapArea *jitArea
		for i := range d.heapAreas {
			heapArea := &d.heapAreas[i]
			if stub.start >= heapArea.start && stub.end <= heapArea.end {
				stubHeapArea = heapArea
				break
			}
		}
		if stubHeapArea == nil {
			log.Warnf("Unable to find heap for stub: pid = %d, stub.start = 0x%x",
				pid, stub.start)
			continue
		}

		// Create and insert a jitArea for the stub.
		stubArea, err := jitAreaForStubArm64(&stub, stubHeapArea, d.rm)
		if err != nil {
			log.Warnf("Failed to create JIT area for stub (pid = %d, stub.start = 0x%x): %v",
				pid, stub.start, err)
			continue
		}
		if err = d.addJitArea(ebpf, pid, stubArea); err != nil {
			log.Warnf("Failed to insert JIT area for stub (pid = %d, stub.start = 0x%x): %v",
				pid, stub.start, err)
			continue
		}
	}
}

func (d *hotspotInstance) SynchronizeMappings(ebpf interpreter.EbpfHandler,
	_ reporter.ExecutableReporter, pr process.Process, _ []process.Mapping) error {
	vmd, err := d.d.GetOrInit(func() (hotspotVMData, error) { return d.d.newVMData(d.rm, d.bias) })
	if err != nil {
		return err
	}

	// Check for permanent errors
	if vmd.err != nil {
		return vmd.err
	}

	// Populate main mappings, if not done previously.
	pid := pr.PID()
	err = d.populateMainMappings(vmd, ebpf, pid)
	if err != nil {
		return err
	}
	if !d.mainMappingsInserted {
		// Not ready yet: try later.
		return nil
	}

	d.updateStubMappings(vmd, ebpf, pid)

	return nil
}

// Symbolize interpreters Hotspot eBPF uwinder given data containing target
// process address and translates it to decorated frames expanding any inlined
// frames to multiple new frames.
func (d *hotspotInstance) Symbolize(frame *host.Frame, frames *libpf.Frames) error {
	if !frame.Type.IsInterpType(libpf.HotSpot) {
		return interpreter.ErrMismatchInterpreterType
	}

	// Extract the HotSpot frame bitfields from the file and line variables
	ptr := libpf.Address(frame.File)
	subtype := uint32(frame.Lineno>>60) & 0xf
	ripOrBci := uint32(frame.Lineno>>32) & 0x0fffffff
	ptrCheck := uint32(frame.Lineno)

	var err error
	sfCounter := successfailurecounter.New(&d.successCount, &d.failCount)
	defer sfCounter.DefaultToFailure()

	switch uint8(subtype) {
	case support.FrameHotspotStub, support.FrameHotspotVtable:
		// These are stub frames that may or may not be interesting
		// to be seen in the trace.
		stubName := d.getStubName(ripOrBci, ptr)
		frames.Append(&libpf.Frame{
			Type:         libpf.HotSpotFrame,
			FunctionName: stubName,
		})
	case support.FrameHotspotInterpreter:
		method, err1 := d.getMethod(ptr, ptrCheck)
		if err1 != nil {
			return err1
		}
		method.symbolize(ripOrBci, d, frames)
	case support.FrameHotspotNative:
		jitinfo, err1 := d.getJITInfo(ptr, ptrCheck)
		if err1 != nil {
			return err1
		}
		err = jitinfo.symbolize(int32(ripOrBci), d, frames)
	default:
		return fmt.Errorf("hotspot frame subtype %v is not supported", subtype)
	}

	if err != nil {
		return err
	}
	sfCounter.ReportSuccess()
	return nil
}
