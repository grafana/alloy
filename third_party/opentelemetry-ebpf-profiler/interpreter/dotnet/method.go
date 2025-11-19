// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"bytes"
	"encoding/binary"
	"fmt"

	log "github.com/sirupsen/logrus"

	npsr "go.opentelemetry.io/ebpf-profiler/nopanicslicereader"
)

type dotnetMethod struct {
	// module is the PE DLL defining this method
	module *peInfo
	// boundInfo is the extracted boundary debug information from coreclr vm.
	boundsInfo []byte
	// methodIndex is the index to MethodDef metadata table defining this method.
	index uint32
	// classification is the coreclr vm categorization of the method type.
	classification uint16
}

// dotnet internal constants which have not changed through the current
// git repository life time, and are unlikely to change.
const (
	// Debug Info boundary info mapping types, as defined in:
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/cordebuginfo.h#L16
	mappingTypeNoMapping = -1
	mappingTypeProlog    = -2
	mappingTypeEpilog    = -3
	mappingTypeMaxValue  = mappingTypeEpilog

	// Debug Info Boundary info's Source Type valid mask
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/cordebuginfo.h#L41
	sourceTypeCallInstruction = 0x10

	// CLR internal debug info flags
	// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/vm/debuginfostore.cpp#L458
	extraDebugInfoPathcPoint = 0x01
	extraDebugInfoRich       = 0x02
)

func (m *dotnetMethod) mapPCOffsetToILOffset(pcOffset uint32, findCall bool) uint32 {
	// FIXME: should there be a small LRU for these?

	// NOTE: The dotnet coreclr optimizing JIT (used when the module is built in Release
	// configuration) does not currently generate reliably DebugInfo. Most importantly
	// it is missing CALL_INSTRUCTION mapping. It seems to generate only best effort
	// STACK_EMPTY mappings which gives only very coarse PC-to-IL mappings. The "wrong
	// line numbers" issue affects also dotnet coreclr itself. See also:
	//   https://github.com/dotnet/runtime/issues/96473#issuecomment-1890383639
	//   https://dotnetdocs.ir/Post/47/wrong-exception-line-number-in-stack-trace-in-release-mode
	r := bytes.NewReader(m.boundsInfo)
	nr := nibbleReader{ByteReader: r}
	numEntries := nr.Uint32()

	log.Debugf("finding method index=%d, pcOffset=%d, callCall=%v, numEntries=%v",
		m.index, pcOffset, findCall, numEntries)

	// Decode Bounds Info portion of DebugInfo
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L289-L310
	nativeOffset := uint32(0)
	ilOffset := uint32(0)
	lastCallILOffset := uint32(0)
	for i := range numEntries {
		nativeOffset += nr.Uint32()
		if findCall && nativeOffset >= pcOffset {
			// If finding call site, always return lastCallILOffset.
			// This will be zero if there are no CALL_INSTRUCTION boundary info.
			log.Debugf("  returning %#x as last call site (next entry's native offset %d)",
				lastCallILOffset, nativeOffset)
			return lastCallILOffset
		} else if nativeOffset > pcOffset {
			log.Debugf("  returning %#x (next entry's native offset %d)",
				ilOffset, nativeOffset)
			return ilOffset
		}

		// Ignore the special value for IL offset. The prolog and epilog values
		// are often emitted for same native offset together with the IL offset.
		// This allows epilog to point to the actual IL ret instruction.
		encodedILOffset := int32(nr.Uint32()) + mappingTypeMaxValue
		if encodedILOffset >= 0 {
			ilOffset = uint32(encodedILOffset)
		}
		sourceFlags := nr.Uint32()
		if sourceFlags&sourceTypeCallInstruction != 0 {
			lastCallILOffset = ilOffset
		}

		// NOTE: _DEBUG builds could have a 0xA nibble to identify row change.
		log.Debugf(" %3d, native %3d -> IL %#03x, sourceFlags %#x",
			i, nativeOffset, ilOffset, sourceFlags)
	}
	return uint32(0)
}

func (m *dotnetMethod) dumpBounds() {
	r := bytes.NewReader(m.boundsInfo)
	nr := nibbleReader{ByteReader: r}
	numEntries := nr.Uint32()

	log.Debugf("dumping method index=%d, numEntries=%v", m.index, numEntries)

	// Decode Bounds Info portion of DebugInfo
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L289-L310
	nativeOffset := uint32(0)
	for i := range numEntries {
		nativeOffset += nr.Uint32()
		ilOffset := uint32(int32(nr.Uint32()) + mappingTypeMaxValue)
		sourceFlags := nr.Uint32()
		// NOTE: _DEBUG builds could have a 0xA nibble to identify row change.

		log.Debugf(" %3d, native %3d -> IL %#03x, sourceFlags %#x",
			i, nativeOffset, ilOffset, sourceFlags)
	}
}

func dumpRichDebugInfo(richInfo []byte) {
	nr := nibbleReader{ByteReader: bytes.NewReader(richInfo)}
	numInlineTree := nr.Uint32()
	numRichOffsets := nr.Uint32()
	log.Debugf("debug info: rich debug %d bytes, %d inlines, %d offsets",
		len(richInfo), numInlineTree, numRichOffsets)

	// Decode Rich Debug info's Inline Tree Nodes
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L404-L429
	var ilOffset, child, sibling int32
	for range numInlineTree {
		ptr := nr.Ptr()
		ilOffset += nr.Int32()
		child += nr.Int32()
		sibling += nr.Int32()
		log.Debugf("  il %03d child %d sibling %x handle %x",
			ilOffset, child, sibling, ptr)
	}

	// Decode Rich Debug info's Offset Mappings
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L431-L456
	nativeOffset := uint32(0)
	ilOffset = 0
	inlinee := int32(0)
	for range numRichOffsets {
		nativeOffset += nr.Uint32()
		inlinee += nr.Int32()
		ilOffset += nr.Int32()
		sourceFlags := nr.Uint32()
		log.Debugf("  native %d IL %x inlinee %d flags %x",
			nativeOffset, ilOffset, inlinee, sourceFlags)
	}
}

// Read and parse the dotnet coreclr DebugInfo structure
// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L711
func (m *dotnetMethod) readDebugInfo(r *cachingReader, d *dotnetData) error {
	// The Flags byte is optional depending on build options. Namely FEATURE_ON_STACK_REPLACEMENT
	// enables it always, which is always enabled for x86 and arm64.
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/codeman.cpp#L3786-L3804
	flags, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read flags: %w", err)
	}
	if flags&^(extraDebugInfoPathcPoint|extraDebugInfoRich) != 0 {
		return fmt.Errorf("flags (%#x) not supported", flags)
	}
	if flags&extraDebugInfoPathcPoint != 0 {
		// skip PatchpointInfo
		// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L741-L746
		// https://github.com/dotnet/runtime/blob/v7.0.15/src/coreclr/inc/patchpointinfo.h#L29-L35
		vms := &d.vmStructs
		patchpointInfo := make([]byte, vms.PatchpointInfo.SizeOf)
		if _, err = r.Read(patchpointInfo); err != nil {
			return fmt.Errorf("failed to read patchpoint info: %w", err)
		}
		numLocals := npsr.Uint32(patchpointInfo, vms.PatchpointInfo.NumberOfLocals)
		r.Skip(int(numLocals * 4))
		log.Debugf("debug info: skipped patchpoint info with %d locals", numLocals)
	}
	if flags&extraDebugInfoRich != 0 {
		// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L748-L754
		// export COMPlus_RichDebugInfo=1 (to enable generation of this information)
		lengthBytes := make([]byte, 4)
		if _, err = r.Read(lengthBytes); err != nil {
			return fmt.Errorf("failed to read rich debug: %w", err)
		}

		length := binary.LittleEndian.Uint32(lengthBytes)
		if dumpDebugInfo {
			richInfo := make([]byte, length)
			_, err = r.Read(richInfo)
			if err != nil {
				return fmt.Errorf("failed to read rich debug: %w", err)
			}
			dumpRichDebugInfo(richInfo)
		} else {
			r.Skip(int(length))
		}
		// FIXME: implement support for RichDebugInfo to decode inlining info
		// In dotnet7, the RichDebugInfo was added back as experimental opt-in
		// feature, but mentioning the format may change.
		//   https://github.com/dotnet/runtime/pull/71263
		// We have open issue to make the RichDebugInfo usable for profilers:
		//   https://github.com/dotnet/runtime/issues/96473
	}

	// Decode the DebugInfo header
	// https://github.com/dotnet/runtime/blob/main/src/coreclr/vm/debuginfostore.cpp#L759-L765
	nr := nibbleReader{ByteReader: r}
	numBytesBounds := nr.Uint32()
	numBytesVars := nr.Uint32()
	nr.AlignToBytes()
	if err := nr.Error(); err != nil {
		return fmt.Errorf("failed to read bounds header: %w", err)
	}
	log.Debugf("debug info: bounds size %d, vars size %d", numBytesBounds, numBytesVars)
	if numBytesBounds > maxBoundsSize {
		return fmt.Errorf("boundary debug info size %d is too large", numBytesBounds)
	}

	// Extract the boundary information blob
	m.boundsInfo = make([]byte, numBytesBounds)
	if _, err := r.Read(m.boundsInfo); err != nil {
		m.boundsInfo = nil
		return fmt.Errorf("failed to read bounds: %w", err)
	}
	if dumpDebugInfo {
		m.dumpBounds()
	}
	return nil
}
