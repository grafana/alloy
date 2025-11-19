// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"sync/atomic"
	"time"

	"github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
	"go.opentelemetry.io/ebpf-profiler/libpf/readatbuf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/util"
)

const (
	// Maximum size of the LRU cache holding the executables' PE information.
	peInfoCacheSize = 16384

	// TTL of entries in the LRU cache holding the executables' PE information.
	peInfoCacheTTL = 6 * time.Hour
)

// OptionalHeader32 is the IMAGE_OPTIONAL_HEADER32 without its Magic or DataDirectory
// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-image_optional_header32
type OptionalHeader32 struct {
	MajorLinkerVersion          uint8
	MinorLinkerVersion          uint8
	SizeOfCode                  uint32
	SizeOfInitializedData       uint32
	SizeOfUninitializedData     uint32
	AddressOfEntryPoint         uint32
	BaseOfCode                  uint32
	BaseOfData                  uint32
	ImageBase                   uint32
	SectionAlignment            uint32
	FileAlignment               uint32
	MajorOperatingSystemVersion uint16
	MinorOperatingSystemVersion uint16
	MajorImageVersion           uint16
	MinorImageVersion           uint16
	MajorSubsystemVersion       uint16
	MinorSubsystemVersion       uint16
	Win32VersionValue           uint32
	SizeOfImage                 uint32
	SizeOfHeaders               uint32
	CheckSum                    uint32
	Subsystem                   uint16
	DllCharacteristics          uint16
	SizeOfStackReserve          uint32
	SizeOfStackCommit           uint32
	SizeOfHeapReserve           uint32
	SizeOfHeapCommit            uint32
	LoaderFlags                 uint32
	NumberOfRvaAndSizes         uint32
}

// OptionalHeader64 is the IMAGE_OPTIONAL_HEADER64 without its Magic or DataDirectory
// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-image_optional_header64
type OptionalHeader64 struct {
	MajorLinkerVersion          uint8
	MinorLinkerVersion          uint8
	SizeOfCode                  uint32
	SizeOfInitializedData       uint32
	SizeOfUninitializedData     uint32
	AddressOfEntryPoint         uint32
	BaseOfCode                  uint32
	ImageBase                   uint64
	SectionAlignment            uint32
	FileAlignment               uint32
	MajorOperatingSystemVersion uint16
	MinorOperatingSystemVersion uint16
	MajorImageVersion           uint16
	MinorImageVersion           uint16
	MajorSubsystemVersion       uint16
	MinorSubsystemVersion       uint16
	Win32VersionValue           uint32
	SizeOfImage                 uint32
	SizeOfHeaders               uint32
	CheckSum                    uint32
	Subsystem                   uint16
	DllCharacteristics          uint16
	SizeOfStackReserve          uint64
	SizeOfStackCommit           uint64
	SizeOfHeapReserve           uint64
	SizeOfHeapCommit            uint64
	LoaderFlags                 uint32
	NumberOfRvaAndSizes         uint32
}

// CLIHeader is the ECMA-335 II.25.3.3 CLI header
type CLIHeader struct {
	SizeOfHeader            uint32
	MajorRuntimeVersion     uint16
	MinorRuntimeVersion     uint16
	MetaData                pe.DataDirectory
	Flags                   uint32
	EntryPointToken         uint32
	Resources               pe.DataDirectory
	StrongNameSignature     pe.DataDirectory
	CodeManagerTable        pe.DataDirectory
	VTableFixups            pe.DataDirectory
	ExportAddressTableJumps pe.DataDirectory
	ManagedNativeHeader     pe.DataDirectory
}

const (
	// The image contains native code.
	// ECMA-335 II.25.3.3.1 Runtime flags
	// R2RFMT "PE Headers and CLI Headers"
	comimageFlagsILLibrary = 0x04

	// R2R RuntimeFunctions section identifier
	r2rSectionRuntimeFunctions = 102
	// R2R MethodDefEntryPoints section identifier
	r2rSectionMethodDefEntryPoints = 103
)

// ReadyToRunHeader is the R2RFMT READYTORUN_HEADER + READYTORUN_CORE_HEADER
type ReadyToRunHeader struct {
	Signature    uint32
	MajorVersion uint16
	MinorVersion uint16
	Flags        uint32
	NumSections  uint32
}

// ReadyToRunSection is the R2RFMT READYTORUN_SECTION
type ReadyToRunSection struct {
	Type    uint32
	Section pe.DataDirectory
}

// ReadyToRunRuntimeFunction is the R2RFMT RUNTIME_FUNCTION for x86_64
type ReadyToRunRuntimeFunction struct {
	StartRVA uint32
	EndRVA   uint32
	GCInfo   uint32
}

// MetadataRoot is the ECMA-335 II.24.2.1 Metadata root (non-variable length header)
type MetadataRoot struct {
	Signature    uint32
	MajorVersion uint16
	MinorVersion uint16
	Reserved     uint32
	Length       uint32
}

// StreamHeader is the ECMA-335 II.24.2.2 Stream header (non-variable length header)
type StreamHeader struct {
	Offset uint32
	Size   uint32
}

// table* variables are ECMA-335 II.22 defined Metadata table numbers
const (
	tableModule                 = 0x00
	tableTypeRef                = 0x01
	tableTypeDef                = 0x02
	tableFieldPtr               = 0x03
	tableField                  = 0x04
	tableMethodPtr              = 0x05
	tableMethodDef              = 0x06
	tableParam                  = 0x08
	tableInterfaceImpl          = 0x09
	tableMemberRef              = 0x0a
	tableConstant               = 0x0b
	tableCustomAttribute        = 0x0c
	tableFieldMarshal           = 0x0d
	tableDeclSecurity           = 0x0e
	tableClassLayout            = 0x0f
	tableFieldLayout            = 0x10
	tableStandAloneSig          = 0x11
	tableEventMap               = 0x12
	tableEvent                  = 0x14
	tablePropertyMap            = 0x15
	tableProperty               = 0x17
	tableMethodSemantics        = 0x18
	tableMethodImpl             = 0x19
	tableModuleRef              = 0x1a
	tableTypeSpec               = 0x1b
	tableImplMap                = 0x1c
	tableFieldRVA               = 0x1d
	tableAssembly               = 0x20
	tableAssemblyProcessor      = 0x21
	tableAssemblyOS             = 0x22
	tableAssemblyRef            = 0x23
	tableAssemblyRefProcessor   = 0x24
	tableAssemblyRefOS          = 0x25
	tableFile                   = 0x26
	tableExportedType           = 0x27
	tableManifestResource       = 0x28
	tableNestedClass            = 0x29
	tableGenericParam           = 0x2a
	tableMethodSpec             = 0x2b
	tableGenericParamConstraint = 0x2c
)

// peTypeSpec is the information we need to store from a TypeDef entry for symbolization
type peTypeSpec struct {
	namespaceIdx   uint32
	typeNameIdx    uint32
	methodIdx      uint32
	enclosingClass uint32
}

// peMethodSpec is the information we need to store from a MethodDef entry for symbolization
type peMethodSpec struct {
	methodNameIdx uint32
	startRVA      uint32
}

// index* variables are the index key types used as metadata table column values.
// These are internal to our code.
const (
	// Indexes to heap as defined in ECMA-335 II.24.2.[345]
	indexString = iota
	indexGUID
	indexBlob
	// Coded indexes as defined in ECMA-335 II.24.2.6
	indexResolutionScope
	indexTypeDefOrRef
	indexMethodDefOrRef
	indexMemberRefParent
	indexHasConstant
	indexHasCustomAttribute
	indexCustomAttributeType
	indexHasFieldMarshal
	indexHasDeclSecurity
	indexHasSemantics
	indexMemberForwarded
	indexImplementation
	// Indexes to ECMA-335 II.22 defined tables
	indexTypeDef
	indexField
	indexMethodDef
	indexParam
	indexEvent
	indexProperty
	indexModuleRef
	indexCount
)

// peInfo is the information we need to cache from a Dotnet PE file for symbolization
type peInfo struct {
	err          error
	lastModified int64
	file         libpf.FrameMappingFile
	simpleName   libpf.String
	guid         string
	typeSpecs    []peTypeSpec
	methodSpecs  []peMethodSpec
	sizeOfImage  uint32

	// strings contains the preloaded strings from dotnet string heap.
	// If this consumes too much memory, this could be converted to LRU and on-demand
	// populated by reading the strings from attached process memory.
	strings map[uint32]libpf.String
}

// peParser contains the needed data when reading and parsing the dotnet data from a PE file.
type peParser struct {
	info    *peInfo
	headers []byte

	io.ReaderAt
	io.ReadSeeker

	peBase int64

	err error

	nt       pe.FileHeader
	cli      pe.DataDirectory
	sections []pe.SectionHeader32

	indexSizes [indexCount]int
	tableRows  [64]uint32

	dotnetTables  io.ReadSeeker
	dotnetStrings io.ReaderAt
	dotnetGUID    io.ReaderAt

	r2rFunctions io.ReadSeeker
}

func (pp *peParser) parseMZ() error {
	// ECMA-335 II.25.2.1 "MS-DOS header" has additional requirements for this header.

	// The first 96 contains the MZ header
	if pp.headers[0] != 'M' || pp.headers[1] != 'Z' {
		return fmt.Errorf("invalid MZ header: %x", pp.headers[0:2])
	}

	// PE signature offset
	signoff := int64(binary.LittleEndian.Uint32(pp.headers[0x3c:]))
	if signoff >= int64(len(pp.headers)-4) {
		return fmt.Errorf("invalid PE offset: %x", signoff)
	}

	if !bytes.Equal(pp.headers[signoff:signoff+4], []byte{'P', 'E', 0, 0}) {
		return fmt.Errorf("invalid PE magic: %x", pp.headers[signoff:signoff+4])
	}
	pp.peBase = signoff + 4
	return nil
}

func (pp *peParser) parsePE() error {
	// ECMA-335 II.25.2.2 "PE File header" defines this
	_, _ = pp.Seek(pp.peBase, io.SeekStart)
	if err := binary.Read(pp, binary.LittleEndian, &pp.nt); err != nil {
		return err
	}

	// According to ECMA-335 the Machine should be always IMAGE_FILE_MACHINE_I386.
	// R2RFMT "PE Headers and CLI Headers", Machine is set to platform for which
	// Ready to Run code has been generated.
	switch pp.nt.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64,
		pe.IMAGE_FILE_MACHINE_I386, // According to ECMA spec always this
		0xfd1d:                     // Seen on dotnet internal .dlls
		// ok
	default:
		return fmt.Errorf("unrecognized PE machine: %#x", pp.nt.Machine)
	}
	return nil
}

func (pp *peParser) parseOptionalHeader() error {
	// ECMA-335 II.25.2.3 "PE optional header" defines requirements for this header
	if _, err := pp.Seek(pp.peBase+int64(binary.Size(pp.nt)), io.SeekStart); err != nil {
		return err
	}

	var magic uint16
	if err := binary.Read(pp, binary.LittleEndian, &magic); err != nil {
		return err
	}

	// ECMA-335 II.25.2.3.1 requires always a PE32 (0x10b) header, but the dotnet clr
	// internal PE files have actually a PE32+ header.
	var numDirectories, sizeHeaders uint32
	switch magic {
	case 0x10b: // PE32
		var opt32 OptionalHeader32
		if err := binary.Read(pp, binary.LittleEndian, &opt32); err != nil {
			return err
		}
		sizeHeaders = opt32.SizeOfHeaders
		numDirectories = opt32.NumberOfRvaAndSizes
		pp.info.sizeOfImage = opt32.SizeOfImage
	case 0x20b: // PE32+ (PE64)
		var opt64 OptionalHeader64
		if err := binary.Read(pp, binary.LittleEndian, &opt64); err != nil {
			return err
		}
		sizeHeaders = opt64.SizeOfHeaders
		numDirectories = opt64.NumberOfRvaAndSizes
		pp.info.sizeOfImage = opt64.SizeOfImage
	default:
		return fmt.Errorf("invalid optional header magic: %x", magic)
	}
	if sizeHeaders > uint32(len(pp.headers)) {
		return fmt.Errorf("invalid header size: %d", sizeHeaders)
	}
	if numDirectories < 0x10 {
		return fmt.Errorf("invalid unmber of data directories: %d", numDirectories)
	}

	// ECMA-335 II.25.2.3.3 "PE header data directories" defines the data directory
	// indexes. Slot 14 is the "CLI Header" data directory entry.
	if _, err := pp.Seek(14*int64(binary.Size(pe.DataDirectory{})), io.SeekCurrent); err != nil {
		return err
	}
	if err := binary.Read(pp, binary.LittleEndian, &pp.cli); err != nil {
		return err
	}

	pp.sections = make([]pe.SectionHeader32, pp.nt.NumberOfSections)
	if _, err := pp.Seek(int64(numDirectories-15)*int64(binary.Size(pe.DataDirectory{})),
		io.SeekCurrent); err != nil {
		return err
	}

	if err := binary.Read(pp, binary.LittleEndian, pp.sections); err != nil {
		return err
	}

	// Check sections headers that they look sane to the extent we care
	for index, section := range pp.sections {
		if section.VirtualSize >= 0x10000000 {
			return fmt.Errorf("section %d, virtual size is huge (%#x)",
				index, section.VirtualSize)
		}
		if section.VirtualAddress >= 0x10000000 {
			return fmt.Errorf("section %d, relative virtual address (RVA) is huge (%#x)",
				index, section.VirtualAddress)
		}
	}

	return nil
}

// getRVASectionReader() find the PE Section containing the requested DataDirectory and
// creates a SectionReader for the range. This is done by searching for the matching
// PE Section mapping and converting the Relative Virtual Address (RVA) to file offset.
func (pp *peParser) getRVASectionReader(dd pe.DataDirectory) (*io.SectionReader, error) {
	for _, s := range pp.sections {
		if dd.VirtualAddress >= s.VirtualAddress &&
			dd.VirtualAddress+dd.Size <= s.VirtualAddress+s.VirtualSize {
			return io.NewSectionReader(pp.ReaderAt,
				int64(dd.VirtualAddress)-int64(s.VirtualAddress)+int64(s.PointerToRawData),
				int64(dd.Size)), nil
		}
	}
	return nil, fmt.Errorf("unable to find section for data at %#x-%#x",
		dd.VirtualAddress, dd.VirtualAddress+dd.Size)
}

func roundUp(value, alignment uint32) uint32 {
	return (value + alignment - 1) &^ (alignment - 1)
}

func (pp *peParser) parseR2RMethodDefs(table pe.DataDirectory) error {
	r, err := pp.getRVASectionReader(table)
	if err != nil {
		return err
	}
	nr := nativeReader{ReaderAt: r}
	prevIndex := uint32(0)
	prevRVA := uint32(0)

	// The ready-to-run MethodDefs table is a lookup table indexed with MethodDef index,
	// and the data contains R2R RuntimeFunction table index (among other things).
	// The callback will get monotonic MethodDef index, and monotonic startRVA.
	return nr.WalkTable(func(index uint32, offset int64) error {
		id, _, err := nr.Uint(offset)
		if err != nil {
			return err
		}
		// The entry decoding is from:
		// https://github.com/dotnet/runtime/blob/v8.0.0/src/coreclr/vm/readytoruninfo.cpp#L1181
		if id&1 != 0 {
			id >>= 2
		} else {
			id >>= 1
		}
		// id is index to the RuntimeFunctions table.
		// Read the Function start address.
		var f ReadyToRunRuntimeFunction
		_, err = pp.r2rFunctions.Seek(int64(id*uint32(binary.Size(f))), io.SeekStart)
		if err != nil {
			return err
		}
		if err := binary.Read(pp.r2rFunctions, binary.LittleEndian, &f); err != nil {
			return err
		}
		// Shift by one, so that the methods without r2r implementation can
		// be inserted in-between valid RVAs
		startRVA := f.StartRVA << 1
		if startRVA < prevRVA {
			return fmt.Errorf("non-monotonic R2R code RVA: %x < %x",
				startRVA, prevRVA)
		}
		prevRVA = startRVA
		for i := prevIndex + 1; i < index; i++ {
			pp.info.methodSpecs[i].startRVA = startRVA - 1
		}
		// Record the Start RVA
		pp.info.methodSpecs[index].startRVA = startRVA
		prevIndex = index
		return nil
	})
}

// parseR2R reads the Read-To-Run data directory needed for symbolization
func (pp *peParser) parseR2R(hdr pe.DataDirectory) error {
	var r2r ReadyToRunHeader

	r, err := pp.getRVASectionReader(hdr)
	if err != nil {
		return err
	}
	if err = binary.Read(r, binary.LittleEndian, &r2r); err != nil {
		return err
	}
	if r2r.Signature != 0x00525452 {
		return nil
	}
	// Walk the Sections. See R2RFMT READYTORUN_SECTION. The array is
	// sorted by section Type to allow binary search.
	for i := uint32(0); i < r2r.NumSections; i++ {
		var s ReadyToRunSection
		if err = binary.Read(r, binary.LittleEndian, &s); err != nil {
			return err
		}
		switch s.Type {
		case r2rSectionRuntimeFunctions:
			pp.r2rFunctions, err = pp.getRVASectionReader(s.Section)
			if err != nil {
				return err
			}
		case r2rSectionMethodDefEntryPoints:
			return pp.parseR2RMethodDefs(s.Section)
		}
	}
	return nil
}

func (pp *peParser) parseCLI() error {
	r, err := pp.getRVASectionReader(pp.cli)
	if err != nil {
		return err
	}

	// Read the data from ECMA-335 II.25.3.3 CLI header
	var cliHeader CLIHeader
	if err = binary.Read(r, binary.LittleEndian, &cliHeader); err != nil {
		return err
	}

	// Read and parse the data from ECMA-335 II.24.2.1 Metadata root
	var metadataRoot MetadataRoot
	r, err = pp.getRVASectionReader(cliHeader.MetaData)
	if err != nil {
		return err
	}
	if err = binary.Read(r, binary.LittleEndian, &metadataRoot); err != nil {
		return err
	}
	if metadataRoot.Signature != 0x424A5342 {
		return fmt.Errorf("invalid metadata signature %#x", metadataRoot.Signature)
	}
	if _, err = r.Seek(int64(roundUp(metadataRoot.Length, 4)+2), io.SeekCurrent); err != nil {
		return err
	}

	var numStreams uint16
	if err = binary.Read(r, binary.LittleEndian, &numStreams); err != nil {
		return err
	}
	for i := uint16(0); i < numStreams; i++ {
		// Read and parse the ECMA-335 II.24.2.2 Stream header
		var hdr StreamHeader
		var nameBuf [32]byte
		if err = binary.Read(r, binary.LittleEndian, &hdr); err != nil {
			return err
		}
		name := nameBuf[:]
		for j := 0; j < len(name); j += 4 {
			block := nameBuf[j : j+4]
			if _, err = r.Read(block); err != nil {
				return err
			}
			if n := bytes.IndexByte(block, 0); n >= 0 {
				name = nameBuf[:j+n]
				break
			}
		}
		switch pfunsafe.ToString(name) {
		case "#Strings":
			// ECMA-335 II.24.2.3 #Strings heap
			pp.dotnetStrings = io.NewSectionReader(r, int64(hdr.Offset), int64(hdr.Size))
		case "#GUID":
			// ECMA-335 II.24.2.5 #GUID heap
			pp.dotnetGUID = io.NewSectionReader(r, int64(hdr.Offset), int64(hdr.Size))
		case "#~":
			// ECMA-335 II.24.2.6 #~ stream
			pp.dotnetTables = io.NewSectionReader(r, int64(hdr.Offset), int64(hdr.Size))
		}
	}

	if err = pp.parseTables(); err != nil {
		return err
	}

	// Check for R2R header
	if cliHeader.Flags&comimageFlagsILLibrary != 0 {
		if err = pp.parseR2R(cliHeader.ManagedNativeHeader); err != nil {
			return err
		}
	}

	return nil
}

func (pp *peParser) readDotnetString(offs uint32) libpf.String {
	// Read a string from the ECMA-335 II.24.2.3 #Strings heap
	if offs == 0 {
		return libpf.NullString
	}

	// Zero terminated string. Assume maximum length of 1024 bytes.
	// But read it in small chunks to make good use of the readatbuf.
	var str [1024]byte
	chunkSize := 128
	for i := 0; i < len(str); i += chunkSize {
		chunk := str[i : i+chunkSize]
		n, err := pp.dotnetStrings.ReadAt(chunk, int64(offs)+int64(i))
		if n == 0 && err != nil {
			return libpf.NullString
		}

		zeroIdx := bytes.IndexByte(chunk[:n], 0)
		if zeroIdx >= 0 {
			return libpf.Intern(pfunsafe.ToString(str[:i+zeroIdx]))
		}
	}

	// Likely broken string.
	return libpf.NullString
}

func (pp *peParser) readDotnetGUID(offs uint32) string {
	// Read a GUID from the ECMA-335 II.24.2.5 #GUID heap
	if offs == 0 {
		return ""
	}

	var guid [16]byte
	if _, err := pp.dotnetGUID.ReadAt(guid[:], int64(offs-1)*16); err != nil {
		return ""
	}

	// Format as a GUID string
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.LittleEndian.Uint32(guid[:4]),
		binary.LittleEndian.Uint16(guid[4:6]),
		binary.LittleEndian.Uint16(guid[6:8]),
		guid[8:10],
		guid[10:])
}

func (pp *peParser) preloadString(heapIndex uint32) {
	// String index is well known empty string
	if heapIndex == 0 {
		return
	}
	// Check if already loaded
	if _, ok := pp.info.strings[heapIndex]; ok {
		return
	}
	pp.info.strings[heapIndex] = pp.readDotnetString(heapIndex)
}

func (pp *peParser) skipDotnetBytes(n int) {
	if n == 0 || pp.err != nil {
		return
	}
	_, pp.err = pp.dotnetTables.Seek(int64(n), io.SeekCurrent)
}

func (pp *peParser) readDotnetIndex(kind int) uint32 {
	if pp.err != nil {
		return 0
	}
	switch pp.indexSizes[kind] {
	case 2:
		var value uint16
		if pp.err = binary.Read(pp.dotnetTables, binary.LittleEndian, &value); pp.err != nil {
			return 0
		}
		return uint32(value)
	case 4:
		var value uint32
		if pp.err = binary.Read(pp.dotnetTables, binary.LittleEndian, &value); pp.err != nil {
			return 0
		}
		return value
	}
	pp.err = fmt.Errorf("tried to read index (%d) value with invalid size (%d)",
		kind, pp.indexSizes[kind])
	return 0
}

// parseModuleTable parses an ECMA-335 II.22.30 Module table
func (pp *peParser) parseModuleTable() {
	// Generation  a 2-byte value, reserved, shall be zero
	// Name        an index into the String heap
	// Mvid        an index into the Guid heap; differs between two versions of the same module
	// EncID       an index into the Guid heap; reserved, shall be zero
	// EncBaseID   an index into the Guid heap; reserved, shall be zero
	for i := uint32(0); i < pp.tableRows[tableModule]; i++ {
		pp.skipDotnetBytes(2)
		nameIdx := pp.readDotnetIndex(indexString)
		guidIdx := pp.readDotnetIndex(indexGUID)
		pp.skipDotnetBytes(2 * pp.indexSizes[indexGUID])

		pp.info.simpleName = pp.readDotnetString(nameIdx)
		pp.info.guid = pp.readDotnetGUID(guidIdx)
	}
}

// preloadTypeSpecStrings preload the strings for given TypeDef entry
func (pp *peParser) preloadTypeSpecStrings(spec *peTypeSpec) {
	if spec.methodIdx < pp.tableRows[tableMethodDef] {
		pp.preloadString(spec.namespaceIdx)
		pp.preloadString(spec.typeNameIdx)
	}
}

// parseTypeDef parses an ECMA-335 II.22.37 TypeDef table
func (pp *peParser) parseTypeDef() {
	// Flags          a 4-byte bitmask of type TypeAttributes, §II.23.1.15
	// TypeName       an index into the String heap
	// TypeNamespace  an index into the String heap
	// Extends        a TypeDefOrRef (§II.24.2.6) coded index
	// FieldList      an index into the Field table; first Fields owned by this Type
	// MethodList     an index into the MethodDef table; first Method owned by this Type

	specs := make([]peTypeSpec, 0, pp.tableRows[tableTypeDef])

	// NOTE: We could probably not load the rows where MethodList is same as the next
	// entry as it is a type without Methods. We also do lookups from symbolization
	// via binary search using the methodIdx field. However, the NestedClass table
	// will contain direct indexes to this table, so we would need to record the index
	// or do the elimination later during the load - so perhaps its not worth while.

	prevEntry := peTypeSpec{}
	for i := uint32(0); i < pp.tableRows[tableTypeDef]; i++ {
		pp.skipDotnetBytes(4)
		typeNameIdx := pp.readDotnetIndex(indexString)
		namespaceIdx := pp.readDotnetIndex(indexString)
		pp.skipDotnetBytes(pp.indexSizes[indexTypeDefOrRef] + pp.indexSizes[indexField])
		methodIdx := pp.readDotnetIndex(indexMethodDef)

		if prevEntry.methodIdx != methodIdx {
			pp.preloadTypeSpecStrings(&prevEntry)
		}

		prevEntry = peTypeSpec{
			namespaceIdx: namespaceIdx,
			typeNameIdx:  typeNameIdx,
			methodIdx:    methodIdx,
		}
		specs = append(specs, prevEntry)
	}
	pp.preloadTypeSpecStrings(&specs[len(specs)-1])

	pp.info.typeSpecs = specs
}

// parseMethodDef parses the ECMA-335 II.22.26 MethodDef table
func (pp *peParser) parseMethodDef() {
	// RVA        a 4-byte constant
	// ImplFlags  a 2-byte bitmask of type MethodImplAttributes, §II.23.1.10
	// Flags      a 2-byte bitmask of type MethodAttributes, §II.23.1.10
	// Name       an index into the String heap
	// Signature  an index into the Blob heap
	// ParamList  an index into the Param table

	specs := make([]peMethodSpec, 0, pp.tableRows[tableMethodDef])

	for i := uint32(0); i < pp.tableRows[tableMethodDef]; i++ {
		pp.skipDotnetBytes(4 + 2 + 2)
		nameIdx := pp.readDotnetIndex(indexString)
		pp.skipDotnetBytes(pp.indexSizes[indexBlob] + pp.indexSizes[indexParam])

		pp.preloadString(nameIdx)
		specs = append(specs, peMethodSpec{methodNameIdx: nameIdx})
	}
	pp.info.methodSpecs = specs
}

// parseNestedClass parses the ECMA-335 II.22.32 NestedClass table
func (pp *peParser) parseNestedClass() {
	// NestedClass     an index into the TypeDef table
	// EnclosingClass  an index into the TypeDef table

	numTypeDefs := uint32(len(pp.info.typeSpecs))

	for i := uint32(0); i < pp.tableRows[tableNestedClass]; i++ {
		nestedClass := pp.readDotnetIndex(indexTypeDef)
		enclosingClass := pp.readDotnetIndex(indexTypeDef)
		if nestedClass <= 0 || nestedClass > numTypeDefs ||
			enclosingClass <= 0 || enclosingClass > numTypeDefs {
			// Invalid indexes
			pp.err = fmt.Errorf("invalid NestedClass row %d: indexes (%d/%d) vs. %d typedefs",
				i, nestedClass, enclosingClass, numTypeDefs)
			return
		}
		pp.info.typeSpecs[nestedClass-1].enclosingClass = enclosingClass
	}
}

// getHeapSize returns the heap size depending if its large or not
func getHeapSize(isLarge bool) int {
	if isLarge {
		return 4
	}
	return 2
}

// getIndexSize calculates the encoded index size given its tag bit size and indexes
// refer to ECMA-335 II.24.2.6 portion about "coded index" on the details.
func (pp *peParser) getIndexSize(tagBits int, indexes []uint) int {
	maxRows := uint32(0)
	for _, index := range indexes {
		if pp.tableRows[index] > maxRows {
			maxRows = pp.tableRows[index]
		}
	}
	if maxRows >= uint32(1<<(16-tagBits)) {
		return 4
	}
	return 2
}

func (pp *peParser) parseTables() error {
	// Parse the ECMA-335 II.24.2.6 #~ stream

	var tablesHeader struct {
		Reserved0    uint32
		MajorVersion uint8
		MinorVersion uint8
		HeapSizes    uint8
		Reserved1    uint8
		Valid        uint64
		Sorted       uint64
		// Rows[] entry for each Valid bit
		// Tables
	}
	r := pp.dotnetTables
	if err := binary.Read(r, binary.LittleEndian, &tablesHeader); err != nil {
		return err
	}
	for i := range 64 {
		if tablesHeader.Valid&(1<<i) == 0 {
			continue
		}
		if err := binary.Read(r, binary.LittleEndian, &pp.tableRows[i]); err != nil {
			return err
		}
	}
	if pp.tableRows[tableModule] != 1 {
		return fmt.Errorf("number of Modules (%d) is unexpected", pp.tableRows[0])
	}

	pp.info.strings = map[uint32]libpf.String{}

	// Precalculate the column sizes we need to know
	pp.indexSizes[indexString] = getHeapSize(tablesHeader.HeapSizes&0x1 != 0)
	pp.indexSizes[indexGUID] = getHeapSize(tablesHeader.HeapSizes&0x2 != 0)
	pp.indexSizes[indexBlob] = getHeapSize(tablesHeader.HeapSizes&0x4 != 0)

	pp.indexSizes[indexResolutionScope] = pp.getIndexSize(2,
		[]uint{tableModule, tableModuleRef, tableAssemblyRef, tableTypeRef})
	pp.indexSizes[indexTypeDefOrRef] = pp.getIndexSize(2,
		[]uint{tableTypeDef, tableTypeRef, tableTypeSpec})
	pp.indexSizes[indexMethodDefOrRef] = pp.getIndexSize(1, []uint{tableMethodDef, tableMemberRef})
	pp.indexSizes[indexMemberRefParent] = pp.getIndexSize(3,
		[]uint{tableTypeDef, tableTypeRef, tableModuleRef, tableMethodDef, tableTypeSpec})
	pp.indexSizes[indexHasConstant] = pp.getIndexSize(2,
		[]uint{tableField, tableParam, tableProperty})
	pp.indexSizes[indexHasCustomAttribute] = pp.getIndexSize(5,
		[]uint{tableMethodDef, tableField, tableTypeRef, tableTypeDef, tableParam,
			tableInterfaceImpl, tableMemberRef, tableModule, tableDeclSecurity,
			tableProperty, tableEvent, tableStandAloneSig, tableModuleRef,
			tableTypeSpec, tableAssembly, tableAssemblyRef, tableFile, tableExportedType,
			tableManifestResource, tableGenericParam, tableGenericParamConstraint,
			tableMethodSpec})
	pp.indexSizes[indexCustomAttributeType] = pp.getIndexSize(3,
		[]uint{tableMethodDef, tableMemberRef})
	pp.indexSizes[indexHasFieldMarshal] = pp.getIndexSize(1, []uint{tableField, tableParam})
	pp.indexSizes[indexHasDeclSecurity] = pp.getIndexSize(2, []uint{tableTypeDef, tableMethodDef,
		tableAssembly})
	pp.indexSizes[indexHasSemantics] = pp.getIndexSize(1, []uint{tableEvent, tableProperty})
	pp.indexSizes[indexMemberForwarded] = pp.getIndexSize(1, []uint{tableField, tableMethodDef})
	pp.indexSizes[indexImplementation] = pp.getIndexSize(2, []uint{tableFile, tableAssemblyRef,
		tableExportedType})

	pp.indexSizes[indexTypeDef] = pp.getIndexSize(0, []uint{tableTypeDef})
	pp.indexSizes[indexField] = pp.getIndexSize(0, []uint{tableField})
	pp.indexSizes[indexMethodDef] = pp.getIndexSize(0, []uint{tableMethodDef})
	pp.indexSizes[indexParam] = pp.getIndexSize(0, []uint{tableParam})
	pp.indexSizes[indexEvent] = pp.getIndexSize(0, []uint{tableEvent})
	pp.indexSizes[indexProperty] = pp.getIndexSize(0, []uint{tableProperty})
	pp.indexSizes[indexModuleRef] = pp.getIndexSize(0, []uint{tableModuleRef})

	// Each table is follows in sequence. Parse the ones we need.
	for tableIndex, rowCount := range pp.tableRows {
		if rowCount == 0 {
			continue
		}

		var rowSize int
		switch tableIndex {
		case tableModule:
			pp.parseModuleTable()
		case tableTypeRef:
			// an ECMA-335 II.22.38 TypeRef table
			// ResolutionScope  a ResolutionScope (§II.24.2.6) coded index
			// TypeName         an index into the String heap
			// TypeNamespace    an index into the String heap
			rowSize = 2*pp.indexSizes[indexString] + pp.indexSizes[indexResolutionScope]
		case tableTypeDef:
			pp.parseTypeDef()
		case tableFieldPtr:
			// Undocumented in ECMA.
			rowSize = pp.indexSizes[indexField]
		case tableField:
			// an ECMA-335 II.22.15 Field table
			// Flags      a 2-byte bitmask of type FieldAttributes, §II.23.1.5
			// Name       an index into the String heap
			// Signature  an index into the Blob heap
			rowSize = 2 + pp.indexSizes[indexString] + pp.indexSizes[indexBlob]
		case tableMethodDef:
			pp.parseMethodDef()
		case tableParam:
			// an ECMA-335 II.22.33 Param table
			// Flags     a 2-byte bitmask of type ParamAttributes, §II.23.1.13
			// Sequence  a 2-byte constant
			// Name      an index into the String heap
			rowSize = 2 + 2 + pp.indexSizes[indexString]
		case tableInterfaceImpl:
			// an ECMA-335 II.22.23 InterfaceImpl table
			// Class      an index into the TypeDef table
			// Interface  a TypeDefOrRef (§II.24.2.6) coded index
			rowSize = pp.indexSizes[indexTypeDef] + pp.indexSizes[indexTypeDefOrRef]
		case tableMemberRef:
			// an ECMA-335 II.22.25 MemberRef table
			// Class      a MemberRefParent (§II.24.2.6) coded index
			// Name       an index into the String heap
			// Signature  an index into the Blob heap
			rowSize = pp.indexSizes[indexMemberRefParent] + pp.indexSizes[indexString] +
				pp.indexSizes[indexBlob]
		case tableConstant:
			// an ECMA-335 II.22.9 Constant table
			// Type     a 1-byte constant, followed by a 1-byte padding zero
			// Padding  a 1-byte padding zero
			// Parent   a HasConstant (§II.24.2.6) coded index
			// Value    an index into the Blob heap
			rowSize = 2 + pp.indexSizes[indexHasConstant] + pp.indexSizes[indexBlob]
		case tableCustomAttribute:
			// an ECMA-335 II.22.34 CustomAttribute table
			// Parent  an associated HasCustomAttribute (§II.24.2.6) coded index
			// Type    a CustomAttributeType (§II.24.2.6) coded index
			// Value   an index into the Blob heap
			rowSize = pp.indexSizes[indexHasCustomAttribute] +
				pp.indexSizes[indexCustomAttributeType] +
				pp.indexSizes[indexBlob]
		case tableFieldMarshal:
			// an ECMA-335 II.22.17 FieldMarshal table
			// Parent      a HasFieldMarshal (§II.24.2.6) coded index
			// NativeType  an index into the Blob heap
			rowSize = pp.indexSizes[indexHasFieldMarshal] + pp.indexSizes[indexBlob]
		case tableDeclSecurity:
			// an ECMA-335 II.22.11 DeclSecurity table
			// Action         a 2-byte value
			// Parent         a HasDeclSecurity (§II.24.2.6) coded index
			// PermissionSet  an index into the Blob heap
			rowSize = 2 + pp.indexSizes[indexHasDeclSecurity] + pp.indexSizes[indexBlob]
		case tableClassLayout:
			// an ECMA-335 II.22.8 ClassLayout table
			// PackingSize  a 2-byte constant
			// ClassSize    a 4-byte constant
			// Parent       an index into the TypeDef table
			rowSize = 6 + pp.indexSizes[indexTypeDef]
		case tableFieldLayout:
			// an ECMA-335 II.22.16 FieldLayout table
			// Offset  a 4-byte constant
			// FIeld   an index into the Field table
			rowSize = 4 + pp.indexSizes[indexField]
		case tableStandAloneSig:
			// an ECMA-335 II.22.36 StandAloneSig table
			// Offset  an index into the Blob heap
			rowSize = pp.indexSizes[indexBlob]
		case tableEventMap:
			// an ECMA-335 II.22.12 EventMap table
			// Parent     an index into the TypeDef table
			// EventList  an index into the Event table
			rowSize = pp.indexSizes[indexTypeDef] + pp.indexSizes[indexEvent]
		case tableEvent:
			// an ECMA-335 II.22.13 Event table
			// EventFlags  a 2-byte bitmask of type EventAttributes, §II.23.1.4
			// Name        an index into the String heap
			// EventType   a TypeDefOrRef (§II.24.2.6) coded index
			rowSize = 2 + pp.indexSizes[indexString] + pp.indexSizes[indexTypeDefOrRef]
		case tablePropertyMap:
			// an ECMA-335 II.22.35 PropertyMap table
			// Parent        an index into the TypeDef table
			// PropertyList  an index into the Property table
			rowSize = pp.indexSizes[indexTypeDef] + pp.indexSizes[indexProperty]
		case tableProperty:
			// an ECMA-335 II.22.34 Property table
			// Flags  a 2-byte bitmask of type PropertyAttributes, §II.23.1.14
			// Name   an index into the String heap
			// Type   an index into the Blob heap
			rowSize = 2 + pp.indexSizes[indexString] + pp.indexSizes[indexBlob]
		case tableMethodSemantics:
			// an ECMA-335 II.22.18 MethodSemantics table
			// Semantics    a 2-byte bitmask of type MethodSemanticsAttributes, §II.23.1.12
			// Method       an index into the MethodDef table
			// Association  a HasSemantics (§II.24.2.6) coded index
			rowSize = 2 + pp.indexSizes[indexMethodDef] + pp.indexSizes[indexHasSemantics]
		case tableMethodImpl:
			// an ECMA-335 II.22.27 MethodImpl table
			// Class              an index into the TypeDef table
			// MethodBody         a MethodDefOrRef (§II.24.2.6) coded index
			// MethodDeclaration  a MethodDefOrRef (§II.24.2.6) coded index
			rowSize = pp.indexSizes[indexTypeDef] + 2*pp.indexSizes[indexMethodDefOrRef]
		case tableModuleRef:
			// an ECMA-335 II.22.31 ModuleRef table
			// Name  an index into the String heap
			rowSize = pp.indexSizes[indexString]
		case tableTypeSpec:
			// an ECMA-335 II.22.39 TypeSpec table
			// Signature  an index into the Blob heap
			rowSize = pp.indexSizes[indexBlob]
		case tableImplMap:
			// an ECMA-335 II.22.22 ImplMap table
			// MappingFlags     a 2-byte bitmask of type PInvokeAttributes, §23.1.8
			// MemberForwarded  a MemberForwarded (§II.24.2.6) coded index
			// ImportName       an index into the String heap
			// ImportScope      an index into the ModuleRef table
			rowSize = 2 + pp.indexSizes[indexMemberForwarded] + pp.indexSizes[indexString] +
				pp.indexSizes[indexModuleRef]
		case tableFieldRVA:
			// an ECMA-335 II.22.18 FieldRVA table
			// RVA    4-byte constant
			// Field  an index into Field table
			rowSize = 4 + pp.indexSizes[indexField]
		case tableAssembly:
			// an ECMA-335 II.22.2 Assembly table
			// HashAlgId       a 4-byte constant of type AssemblyHashAlgorithm, §II.23.1.1
			// MajorVersion    a 2-byte constant
			// MinorVersion    a 2-byte constant
			// BuildNumber     a 2-byte constant
			// RevisionNumber  a 2-byte constant
			// Flags           a 4-byte bitmask of type AssemblyFlags, §II.23.1.2
			// PublicKey       an index into the Blob heap
			// Name            an index into the String heap
			// Culture         an index into the String heap
			rowSize = 16 + pp.indexSizes[indexBlob] + 2*pp.indexSizes[indexString]
		case tableAssemblyProcessor, tableAssemblyOS,
			tableAssemblyRefProcessor, tableAssemblyRefOS:
			// an ECMA-335 II.22.4 AssemblyProcessor table
			// an ECMA-335 II.22.3 AssemblyOS table
			// an ECMA-335 II.22.7 AssemblyRefProcessor table
			// an ECMA-335 II.22.6 AssemblyRefOS table
			// should not be emitted into any PE file
			return fmt.Errorf("metadata table %x should not be in PE", tableIndex)
		case tableAssemblyRef:
			// an ECMA-335 II.22.5 AssemblyRef table
			// MajorVersion      a 2-byte constant
			// MinorVersion      a 2-byte constant
			// BuildNumber       a 2-byte constant
			// RevisionNumber    a 2-byte constant
			// Flags             a 4-byte bitmask of type AssemblyFlags, §II.23.1.2
			// PublicKeyOrToken  an index into the Blob heap (public key or author token)
			// Name              an index into the String heap
			// Culture           an index into the String heap
			// HashValue         an index into the Blob heap
			rowSize = 12 + 2*pp.indexSizes[indexBlob] + 2*pp.indexSizes[indexString]
		case tableFile:
			// an ECMA-335 II.22.19 File table
			// Flags      a 4-byte bitmask of type FileAttributes, §II.23.1.6
			// Name       an index into the String heap
			// HashValue  an index into the Blob heap
			rowSize = 4 + pp.indexSizes[indexBlob] + pp.indexSizes[indexString]
		case tableExportedType:
			// an ECMA-335 II.22.14 ExportedType table
			// Flags           a 4-byte bitmask of type TypeAttributes, §II.23.1.15
			// TypeDefId       a 4-byte index into a TypeDef table
			// TypeName        an index into the String heap
			// TypeNamespace   an index into the String heap
			// Implementation  an Implementation (§II.24.2.6) coded index
			rowSize = 8 + 2*pp.indexSizes[indexString] + pp.indexSizes[indexImplementation]
		case tableManifestResource:
			// an ECMA-335 II.22.24 ManifestResource table
			// Offset          a 4-byte constant
			// Flags           a 4-byte bitmask of type ManifestResourceAttributes, §II.23.1.9
			// Name            an index into the String heap
			// Implementation  an Implementation (§II.24.2.6) coded index
			rowSize = 8 + pp.indexSizes[indexString] + pp.indexSizes[indexImplementation]
		case tableNestedClass:
			pp.parseNestedClass()
		default:
			// Support up to NestedClass which is the last table we care
			if tableIndex > tableNestedClass {
				break
			}
			return fmt.Errorf("metadata table %x not implemented", tableIndex)
		}

		if rowSize != 0 {
			pp.skipDotnetBytes(rowSize * int(rowCount))
		}

		if pp.err != nil {
			return fmt.Errorf("metadata table parsing failed: %w", pp.err)
		}
	}

	return nil
}

func (pp *peParser) parse() error {
	var err error

	// Rest of the code reads the file using RVAs and section mappings
	// Use caching file reader
	if pp.ReaderAt, err = readatbuf.New(pp.ReaderAt, 4096, 4); err != nil {
		return err
	}
	// Dotnet requires currently all headers to fit into 4K
	pp.headers = make([]byte, 4096)
	if _, err = pp.ReadAt(pp.headers, 0); err != nil {
		return fmt.Errorf("failed to read PE header: %v", err)
	}
	pp.ReadSeeker = bytes.NewReader(pp.headers)
	if err = pp.parseMZ(); err != nil {
		return err
	}
	if err = pp.parsePE(); err != nil {
		return err
	}
	if err = pp.parseOptionalHeader(); err != nil {
		return err
	}
	return pp.parseCLI()
}

func (pi *peInfo) resolveMethodName(methodIdx uint32) libpf.String {
	if methodIdx == 0 || methodIdx > uint32(len(pi.methodSpecs)) {
		return libpf.Intern(fmt.Sprintf("<invalid method index %d/%d>",
			methodIdx, len(pi.methodSpecs)))
	}

	idx, ok := slices.BinarySearchFunc(pi.typeSpecs, methodIdx,
		func(typespec peTypeSpec, methodIdx uint32) int {
			if methodIdx < typespec.methodIdx {
				return 1
			}
			if methodIdx > typespec.methodIdx {
				return -1
			}
			return 0
		})
	if !ok {
		idx--
	}

	typeSpec := &pi.typeSpecs[idx]
	typeName := pi.strings[typeSpec.typeNameIdx].String()
	for typeSpec.enclosingClass != 0 {
		enclosingSpec := &pi.typeSpecs[typeSpec.enclosingClass-1]
		typeName = fmt.Sprintf("%s/%s", pi.strings[enclosingSpec.typeNameIdx], typeName)
		typeSpec = enclosingSpec
	}
	methodName := pi.strings[pi.methodSpecs[methodIdx-1].methodNameIdx]
	if typeSpec.namespaceIdx != 0 {
		return libpf.Intern(fmt.Sprintf("%s.%s.%s",
			pi.strings[typeSpec.namespaceIdx],
			typeName, methodName))
	}
	return libpf.Intern(fmt.Sprintf("%s.%s", typeName, methodName))
}

func (pi *peInfo) resolveR2RMethodName(pcRVA uint32) libpf.String {
	idx, ok := slices.BinarySearchFunc(pi.methodSpecs, pcRVA<<1,
		func(methodspec peMethodSpec, pcRVA uint32) int {
			if pcRVA < methodspec.startRVA {
				return 1
			}
			if pcRVA > methodspec.startRVA {
				return -1
			}
			return 0
		})
	if !ok {
		idx--
	}
	return pi.resolveMethodName(uint32(idx + 1))
}

func (pi *peInfo) parse(r io.ReaderAt) error {
	pp := peParser{
		ReaderAt: r,
		info:     pi,
	}
	err := pp.parse()
	if err != nil {
		return err
	}
	return nil
}

type peCache struct {
	// peInfoCacheHit
	peInfoCacheHit  atomic.Uint64
	peInfoCacheMiss atomic.Uint64

	// elfInfoCache provides a cache to quickly retrieve the PE info and fileID for a particular
	// executable. It caches results based on iNode number and device ID. Locked LRU.
	peInfoCache *freelru.LRU[util.OnDiskFileIdentifier, *peInfo]
}

func (pc *peCache) init() {
	peInfoCache, err := freelru.New[util.OnDiskFileIdentifier, *peInfo](peInfoCacheSize,
		util.OnDiskFileIdentifier.Hash32)
	if err != nil {
		panic(fmt.Errorf("unable to create peInfoCache: %v", err))
	}
	peInfoCache.SetLifetime(peInfoCacheTTL)
	pc.peInfoCache = peInfoCache
}

func (pc *peCache) Get(pr process.Process, mapping *process.Mapping) *peInfo {
	key := mapping.GetOnDiskFileIdentifier()
	lastModified := pr.GetMappingFileLastModified(mapping)
	if info, ok := pc.peInfoCache.Get(key); ok && info.lastModified == lastModified {
		// Cached data ok
		pc.peInfoCacheHit.Add(1)
		return info
	}

	// Slow path, calculate all the data and update cache
	pc.peInfoCacheMiss.Add(1)

	file, err := pr.OpenMappingFile(mapping)
	if err != nil {
		info := &peInfo{err: err}
		if !errors.Is(err, os.ErrNotExist) {
			pc.peInfoCache.Add(key, info)
		}
		return info
	}
	defer file.Close()

	fileID, err := pr.CalculateMappingFileID(mapping)
	if err != nil {
		return &peInfo{err: err}
	}

	info := &peInfo{
		lastModified: lastModified,
	}
	info.err = info.parse(file)
	if info.err == nil {
		info.file = libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
			FileID:     fileID,
			FileName:   libpf.Intern(path.Base(mapping.Path.String())),
			GnuBuildID: info.guid,
		})
	}
	pc.peInfoCache.Add(key, info)
	return info
}

var globalPeCache peCache

func init() {
	globalPeCache.init()
}
