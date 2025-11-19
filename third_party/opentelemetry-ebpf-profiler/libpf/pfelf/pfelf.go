// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package pfelf implements functions for processing of ELF files and extracting data from
// them. This file provides convenience functions for golang debug/elf standard library.
package pfelf // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf"

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// HasDWARFData returns true if the provided ELF file contains actionable DWARF debugging
// information.
// This function does not call `elfFile.DWARF()` on purpose, as it can be extremely expensive in
// terms of CPU/memory, possibly uncompressing all data in `.zdebug_` sections.
// This function being used extensively by the indexing service, it is preferable to keep it
// lightweight.
func HasDWARFData(elfFile *elf.File) bool {
	hasBuildID := false
	hasDebugStr := false
	for _, section := range elfFile.Sections {
		// NOBITS indicates that the section is actually empty, regardless of the size in the
		// section header.
		if section.Type == elf.SHT_NOBITS {
			continue
		}

		if section.Name == ".note.gnu.build-id" {
			hasBuildID = true
		}

		if section.Name == ".debug_str" || section.Name == ".zdebug_str" ||
			section.Name == ".debug_str.dwo" {
			hasDebugStr = section.Size > 0
		}

		// Some files have suspicious near-empty, partially stripped sections; consider them as not
		// having DWARF data.
		// The simplest binary gcc 10 can generate ("return 0") has >= 48 bytes for each section.
		// Let's not worry about executables that may not verify this, as they would not be of
		// interest to us.
		if section.Size < 32 {
			continue
		}

		if section.Name == ".debug_info" || section.Name == ".zdebug_info" {
			return true
		}
	}

	// Some alternate debug files only have a .debug_str section. For these we want to return true.
	// Use the absence of program headers and presence of a Build ID as heuristic to identify
	// alternate debug files.
	return len(elfFile.Progs) == 0 && hasBuildID && hasDebugStr
}

var ErrNoDebugLink = errors.New("no debug link")

// ParseDebugLink parses the name and CRC32 of the debug info file from the provided section data.
// Error is returned if the data is malformed.
func ParseDebugLink(data []byte) (linkName string, crc32 int32, err error) {
	strEnd := bytes.IndexByte(data, 0)
	if strEnd < 0 {
		return "", 0, errors.New("malformed debug link, not zero terminated")
	}
	linkName = strings.ToValidUTF8(string(data[:strEnd]), "")

	strEnd++
	// The link contains 0 to 3 bytes of padding after the null character, CRC32 is 32-bit aligned
	crc32StartIdx := strEnd + ((4 - (strEnd & 3)) & 3)
	if crc32StartIdx+4 > len(data) {
		return "", 0, fmt.Errorf("malformed debug link, no CRC32 (len %v, start index %v)",
			len(data), crc32StartIdx)
	}

	linkCRC32 := binary.LittleEndian.Uint32(data[crc32StartIdx : crc32StartIdx+4])

	return linkName, int32(linkCRC32), nil
}

func getSectionData(elfFile *elf.File, sectionName string) ([]byte, error) {
	section := elfFile.Section(sectionName)
	if section == nil {
		return nil, fmt.Errorf("failed to open the %s section", sectionName)
	}
	data, err := section.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read data from section %s: %v", sectionName, err)
	}
	return data, nil
}

// GetDebugLink reads and parses the .gnu_debuglink section of given ELF file.
// Error is returned if the data is malformed. If the link does not exist then
// ErrNoDebugLink is returned.
func GetDebugLink(elfFile *elf.File) (linkName string, crc32 int32, err error) {
	// The .gnu_debuglink section is not always present
	sectionData, err := getSectionData(elfFile, ".gnu_debuglink")
	if err != nil {
		return "", 0, ErrNoDebugLink
	}

	return ParseDebugLink(sectionData)
}

var ErrNoBuildID = errors.New("no build ID")
var ubuntuKernelSignature = regexp.MustCompile(` \(Ubuntu[^)]*\)\n$`)

// GetKernelVersionBytes returns the kernel version from a kernel image, as it appears in
// /proc/version
//
// This makes the assumption that the string is the first one in ".rodata" that starts with
// "Linux version ".
func GetKernelVersionBytes(elfFile *elf.File) ([]byte, error) {
	sectionData, err := getSectionData(elfFile, ".rodata")
	if err != nil {
		return nil, fmt.Errorf("failed to read kernel version: %v", err)
	}

	// Prepend a null character to make sure this is the beginning of a string
	procVersionContents := append([]byte{0x0}, []byte("Linux version ")...)

	startIdx := bytes.Index(sectionData, procVersionContents)
	if startIdx < 0 {
		return nil, errors.New("unable to find Linux version")
	}
	// Skip the null character
	startIdx++
	endIdx := bytes.IndexByte(sectionData[startIdx:], 0x0)
	if endIdx < 0 {
		return nil, errors.New("unable to find Linux version (can't find end of string)")
	}

	versionBytes := sectionData[startIdx : startIdx+endIdx]

	// Ubuntu has some magic sauce that adds an extra signature at the end of the linux_banner
	// string in init/version.c which is being extracted here. We replace it with the empty string
	// to ensure it matches the contents of /proc/version, as extracted by the host agent.
	return ubuntuKernelSignature.ReplaceAllLiteral(versionBytes, []byte{'\n'}), nil
}

// GetBuildID extracts the build ID from the provided ELF file. This is read from
// the .note.gnu.build-id or .notes section of the ELF, and may not exist. If no build ID is present
// an ErrNoBuildID is returned.
func GetBuildID(elfFile *elf.File) (string, error) {
	sectionData, err := getSectionData(elfFile, ".note.gnu.build-id")
	if err != nil {
		sectionData, err = getSectionData(elfFile, ".notes")
		if err != nil {
			return "", ErrNoBuildID
		}
	}

	return getBuildIDFromNotes(sectionData)
}

// getGoBuildIDFromNotes returns the Go build ID from an ELF notes section data.
func getGoBuildIDFromNotes(notes []byte) (string, error) {
	// Identify the Go Build ID with ELF_NOTE_GOBUILDID_TAG (0x4).
	buildID, found, err := getNoteString(notes, "Go", 0x4)
	if err != nil {
		return "", fmt.Errorf("could not determine BuildID: %v", err)
	}
	if !found {
		return "", ErrNoBuildID
	}
	return buildID, nil
}

// GetBuildIDFromNotesFile returns the build ID contained in a file with the format of an ELF notes
// section.
func GetBuildIDFromNotesFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("could not open %s: %w", filePath, err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", filePath, err)
	}
	return getBuildIDFromNotes(data)
}

// getBuildIDFromNotes returns the build ID from an ELF notes section data.
func getBuildIDFromNotes(notes []byte) (string, error) {
	// 0x3 is the "Build ID" type. Not sure where this is standardized.
	buildID, found, err := getNoteHexString(notes, "GNU", 0x3)
	if err != nil {
		return "", fmt.Errorf("could not determine BuildID: %v", err)
	}
	if !found {
		return "", ErrNoBuildID
	}
	return buildID, nil
}

// GetSectionAddress returns the address of an ELF section.
// `found` is set to false if such a section does not exist.
func GetSectionAddress(e *elf.File, sectionName string) (
	addr uint64, found bool, err error) {
	section := e.Section(sectionName)
	if section == nil {
		return 0, false, nil
	}

	return section.Addr, true, nil
}

// getNoteDescBytes returns the bytes contents of an ELF note from a note section, as described
// in the ELF standard in Figure 2-3.
func getNoteDescBytes(sectionBytes []byte, name string, noteType uint32) (
	noteBytes []byte, found bool, err error) {
	// The data stored inside ELF notes is made of one or multiple structs, containing the
	// following fields:
	// 	- namesz	// 32-bit, size of "name"
	// 	- descsz	// 32-bit, size of "desc"
	// 	- type		// 32-bit - 0x3 in case of a BuildID, 0x100 in case of build salt
	// 	- name		// namesz bytes, null terminated
	// 	- desc		// descsz bytes, binary data: the actual contents of the note
	// Because of this structure, the information of the build id starts at the 17th byte.

	// Null terminated string
	nameBytes := append([]byte(name), 0x0)
	noteTypeBytes := make([]byte, 4)

	binary.LittleEndian.PutUint32(noteTypeBytes, noteType)
	noteHeader := append(noteTypeBytes, nameBytes...) //nolint:gocritic

	// Try to find the note in the section
	idx := bytes.Index(sectionBytes, noteHeader)
	if idx == -1 {
		return nil, false, nil
	}
	if idx < 4 { // there needs to be room for descsz
		return nil, false, errors.New("could not read note data size")
	}

	idxDataStart := idx + len(noteHeader)
	idxDataStart += (4 - (idxDataStart & 3)) & 3 // data is 32bit-aligned, round up

	// read descsz and compute the last index of the note data
	dataSize := binary.LittleEndian.Uint32(sectionBytes[idx-4 : idx])
	idxDataEnd := uint64(idxDataStart) + uint64(dataSize)

	// Check sanity (84 is totally arbitrary, as we only use it for Linux ID and (Go) Build ID)
	if idxDataEnd > uint64(len(sectionBytes)) || dataSize > 84 {
		return nil, false, fmt.Errorf(
			"non-sensical note: %d start index: %d, %v end index %d, size %d, section size %d",
			idx, idxDataStart, noteHeader, idxDataEnd, dataSize, len(sectionBytes))
	}
	return sectionBytes[idxDataStart:idxDataEnd], true, nil
}

// getNoteHexString returns the hex string contents of an ELF note from a note section, as described
// in the ELF standard in Figure 2-3.
func getNoteHexString(sectionBytes []byte, name string, noteType uint32) (
	noteHexString string, found bool, err error) {
	noteBytes, found, err := getNoteDescBytes(sectionBytes, name, noteType)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return hex.EncodeToString(noteBytes), true, nil
}

func getNoteString(sectionBytes []byte, name string, noteType uint32) (
	noteString string, found bool, err error) {
	noteBytes, found, err := getNoteDescBytes(sectionBytes, name, noteType)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return string(noteBytes), true, nil
}

func symbolMapFromELFSymbols(syms []elf.Symbol) *libpf.SymbolMap {
	symmap := &libpf.SymbolMap{}
	for _, sym := range syms {
		symmap.Add(libpf.Symbol{
			Name:    libpf.SymbolName(sym.Name),
			Address: libpf.SymbolValue(sym.Value),
			Size:    sym.Size,
		})
	}
	symmap.Finalize()
	return symmap
}

// GetDynamicSymbols gets the dynamic symbols of elf.File and returns them as libpf.SymbolMap for
// fast lookup by address and name.
func GetDynamicSymbols(elfFile *elf.File) (*libpf.SymbolMap, error) {
	syms, err := elfFile.DynamicSymbols()
	if err != nil {
		return nil, err
	}
	return symbolMapFromELFSymbols(syms), nil
}

// IsGoBinary returns true if the provided file is a Go binary (= an ELF file with
// a known Golang section).
func IsGoBinary(file *elf.File) (bool, error) {
	// .go.buildinfo is present since Go 1.13
	sectionFound, err := HasSection(file, ".go.buildinfo")
	if sectionFound || err != nil {
		return sectionFound, err
	}
	// Check also .gopclntab, it's present on older Go files, but not on
	// Go PIE executables built with new Golang
	return HasSection(file, ".gopclntab")
}

// HasSection returns true if the provided file contains a specific section.
func HasSection(file *elf.File, section string) (bool, error) {
	_, sectionFound, err := GetSectionAddress(file, section)
	if err != nil {
		return false, fmt.Errorf("unable to lookup %v section: %v", section, err)
	}

	return sectionFound, nil
}

// HasCodeSection returns true if the file contains at least one non-empty executable code section.
func HasCodeSection(elfFile *elf.File) bool {
	for _, section := range elfFile.Sections {
		// NOBITS indicates that the section is actually empty, regardless of the size specified in
		// the section header.
		// For example, separate debug files generated by objcopy --only-keep-debug do have the same
		// section headers as the original file (with the same sizes), including the +x sections.
		if section.Type == elf.SHT_NOBITS {
			continue
		}

		if section.Flags&elf.SHF_EXECINSTR != 0 && section.Size > 0 {
			return true
		}
	}

	return false
}
