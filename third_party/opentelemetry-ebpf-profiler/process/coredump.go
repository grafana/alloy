// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// This file implements Process interface to access coredump ELF files.

// For NT_FILE coredump mappings: https://www.gabriel.urdhr.fr/2015/05/29/core-file/

package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
	"unsafe"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfunsafe"
)

const (
	// maxNotesSection is the maximum section size for notes.
	maxNotesSection = 16 * 1024 * 1024
)

// CoredumpProcess implements Process interface to ELF coredumps.
type CoredumpProcess struct {
	*pfelf.File

	// files contains coredump's files by name.
	files map[string]*CoredumpFile

	// pid is the original PID from which the coredump was generated.
	pid libpf.PID

	// fname is the the short name of the executable file that was running when the coredump was generated.
	fname string

	// machineData contains the parsed machine data.
	machineData MachineData

	// mappings contains the parsed mappings.
	mappings []Mapping

	// threadInfo contains the parsed thread info.
	threadInfo []ThreadInfo

	// execPhdrPtr points to the main executable's program headers.
	execPhdrPtr libpf.Address

	// hasMusl is set if musl c-library is detected in this coredump. This
	// is needed when opening ELF inside coredump as musl and glibc have
	// differences how they handle the dynamic table.
	hasMusl bool
}

var _ Process = &CoredumpProcess{}

// CoredumpMapping describes a file backed mapping in a coredump.
type CoredumpMapping struct {
	// Prog points to the corresponding PT_LOAD segment.
	Prog *pfelf.Prog
	// File is the backing file for this mapping.
	File *CoredumpFile
	// FileOffset is the offset in the original backing file.
	FileOffset uint64
}

// CoredumpFile contains information about a file mapped into a coredump.
type CoredumpFile struct {
	// parent is the Coredump inside which this file is.
	parent *CoredumpProcess
	// inode is the synthesized inode for this file.
	inode uint64
	// Name is the mapped file's name.
	Name libpf.String
	// Mappings contains mappings regarding this file.
	Mappings []CoredumpMapping
	// Base is the virtual address where this file is loaded.
	Base uint64
}

// ELF64 Note header.
type Note64 struct {
	Namesz, Descsz, Type uint32
}

const (
	NAMESPACE_CORE  = "CORE\x00"
	NAMESPACE_LINUX = "LINUX\x00"

	NT_AUXV         elf.NType = 6
	NT_FILE         elf.NType = 0x46494c45
	NT_ARM_TLS      elf.NType = 0x401
	NT_ARM_PAC_MASK elf.NType = 0x406

	AT_PHDR         = 3
	AT_SYSINFO_EHDR = 33
)

// getAlignedBytes returns 'size' bytes from source slice, and progresses the
// source slice by 'size' aligned to next 4 byte boundary. Used to parse notes.
func getAlignedBytes(rdr io.Reader, size uint32) ([]byte, error) {
	if size == 0 {
		return []byte{}, nil
	}
	alignedSize := (size + 3) &^ 3
	buf := make([]byte, alignedSize)
	if n, err := rdr.Read(buf); n != int(alignedSize) || err != nil {
		return nil, err
	}
	return buf[:size], nil
}

// OpenCoredump opens the named file as a coredump.
func OpenCoredump(name string) (*CoredumpProcess, error) {
	f, err := pfelf.Open(name)
	if err != nil {
		return nil, err
	}
	return OpenCoredumpFile(f)
}

// vaddrMappings is internally used during parsing of coredump structures.
// It's the value of a map indexed with mapping virtual address, and contains the data
// needed to associate data from different coredump data structures to proper internals.
type vaddrMappings struct {
	// prog is the ELF PT_LOAD Program header for this virtual address.
	prog *pfelf.Prog

	// mappingIndex is the mapping's index in processState.Mappings.
	mappingIndex int
}

// OpenCoredumpFile opens the given `pfelf.File` as a coredump.
//
// Ownership of the file is transferred. Closing the coredump closes the underlying file as well.
func OpenCoredumpFile(f *pfelf.File) (*CoredumpProcess, error) {
	cd := &CoredumpProcess{
		File:       f,
		files:      make(map[string]*CoredumpFile),
		mappings:   make([]Mapping, 0, len(f.Progs)),
		threadInfo: make([]ThreadInfo, 0, 8),
	}
	cd.machineData.Machine = cd.Machine

	vaddrToMappings := make(map[uint64]vaddrMappings)

	// First pass of program headers: get PT_LOAD base addresses. The PT_NOTE header is usually
	// before the PT_LOAD ones, so this needs to be done first.
	for i := range f.Progs {
		p := &f.Progs[i]
		if p.Type == elf.PT_LOAD && p.Flags != 0 {
			m := Mapping{
				Vaddr:  p.Vaddr,
				Length: p.Memsz,
				Flags:  p.Flags,
			}
			vaddrToMappings[p.Vaddr] = vaddrMappings{
				prog:         p,
				mappingIndex: len(cd.mappings),
			}
			cd.mappings = append(cd.mappings, m)
		}
	}
	// Parse the coredump specific PT_NOTE program headers we are interested about.
	for i := range f.Progs {
		p := &f.Progs[i]
		if p.Filesz <= 0 {
			continue
		}
		if p.ProgHeader.Type != elf.PT_NOTE {
			continue
		}
		rdr, err := p.DataReader(maxNotesSection)
		if err != nil {
			return nil, err
		}
		var note Note64
		for {
			// Read the note header (name and size lengths), followed by reading
			// their contents. This code advances the position in 'rdr' and should
			// be kept together to parse the notes correctly.
			if _, err = rdr.Read(pfunsafe.FromPointer(&note)); err != nil {
				break
			}
			var nameBytes, desc []byte
			if nameBytes, err = getAlignedBytes(rdr, note.Namesz); err != nil {
				break
			}
			if desc, err = getAlignedBytes(rdr, note.Descsz); err != nil {
				break
			}

			// Parse the note if we are interested in it (skip others).
			name := string(nameBytes)
			ty := elf.NType(note.Type)
			if name == NAMESPACE_CORE {
				switch ty {
				case NT_AUXV:
					cd.parseAuxVector(desc, vaddrToMappings)
				case elf.NT_PRPSINFO:
					err = cd.parseProcessInfo(desc)
				case elf.NT_PRSTATUS:
					err = cd.parseProcessStatus(desc)
				case NT_FILE:
					err = cd.parseMappings(desc, vaddrToMappings)
				}
			} else if name == NAMESPACE_LINUX {
				switch ty {
				case NT_ARM_PAC_MASK:
					err = cd.parseArmPacMask(desc)
				case NT_ARM_TLS:
					err = cd.parseArmTLS(desc)
				}
			}

			if err != nil {
				break
			}
		}
		if err != io.EOF {
			return nil, err
		}
	}

	return cd, nil
}

// MainExecutable gets the file path from the mappings of the main executable.
func (cd *CoredumpProcess) MainExecutable() string {
	if cd.execPhdrPtr == 0 {
		return ""
	}

	for _, file := range cd.files {
		for _, mapping := range file.Mappings {
			if cd.execPhdrPtr >= libpf.Address(mapping.Prog.Vaddr) &&
				cd.execPhdrPtr <= libpf.Address(mapping.Prog.Vaddr+mapping.Prog.Memsz) {
				return file.Name.String()
			}
		}
	}

	return ""
}

// PID implements the Process interface.
func (cd *CoredumpProcess) PID() libpf.PID {
	return cd.pid
}

// GetMachineData implements the Process interface.
func (cd *CoredumpProcess) GetMachineData() MachineData {
	return cd.machineData
}

func (cd *CoredumpProcess) GetProcessMeta(_ MetaConfig) ProcessMeta {
	return ProcessMeta{}
}

func (cd *CoredumpProcess) GetExe() (string, error) {
	return cd.fname, nil
}

// GetMappings implements the Process interface.
func (cd *CoredumpProcess) GetMappings() ([]Mapping, uint32, error) {
	return cd.mappings, 0, nil
}

// GetThreadInfo implements the Process interface.
func (cd *CoredumpProcess) GetThreads() ([]ThreadInfo, error) {
	return cd.threadInfo, nil
}

// OpenMappingFile implements the Process interface.
func (cd *CoredumpProcess) OpenMappingFile(_ *Mapping) (ReadAtCloser, error) {
	// Coredumps do not contain the original backing files.
	return nil, errors.New("coredump does not support opening backing file")
}

// GetMappingFileLastModified implements the Process interface.
func (cd *CoredumpProcess) GetMappingFileLastModified(_ *Mapping) int64 {
	return 0
}

// CalculateMappingFileID implements the Process interface.
func (cd *CoredumpProcess) CalculateMappingFileID(m *Mapping) (libpf.FileID, error) {
	// It is not possible to calculate the real FileID as the section headers
	// are likely missing. So just return a synthesized FileID.
	vaddr := make([]byte, 8)
	binary.LittleEndian.PutUint64(vaddr, m.Vaddr)

	h := fnv.New128a()
	_, _ = h.Write(vaddr)
	_, _ = h.Write([]byte(m.Path.String()))
	return libpf.FileIDFromBytes(h.Sum(nil))
}

// OpenELF implements the ELFOpener and Process interfaces.
func (cd *CoredumpProcess) OpenELF(path string) (*pfelf.File, error) {
	// Fallback to directly returning the data from coredump. This comes with caveats:
	//
	// - The process of loading an ELF binary into memory discards any program regions not marked
	//   as `PT_LOAD`. This means that we won't be able to read sections like `.debug_lines`.
	// - The section table present in memory is typically broken.
	// - Writable data sections won't be in their original state.
	//
	// This essentially means that, during the test run, the HA code is presented with an
	// environment that diverges from the environment it operates in when running on a real system
	// where the original ELF file is available on disk. However, in order to allow keeping around
	// our old test cases from times when we didn't yet bundle the original executables with our
	// tests, we allow this fallback.

	if file, ok := cd.files[path]; ok {
		return file.OpenELF()
	}
	return nil, fmt.Errorf("ELF file `%s` not found", path)
}

// ExtractAsFile implements the Process interface.
func (cd *CoredumpProcess) ExtractAsFile(_ string) (string, error) {
	// Coredumps do not contain the original backing files.
	return "", errors.New("coredump does not support opening backing file")
}

// getFile returns (creating if needed) a matching CoredumpFile for given file name.
func (cd *CoredumpProcess) getFile(name string) *CoredumpFile {
	if cf, ok := cd.files[name]; ok {
		return cf
	}
	if strings.Contains(name, "/ld-musl-") {
		cd.hasMusl = true
	}
	cf := &CoredumpFile{
		parent: cd,
		inode:  uint64(len(cd.files) + 1),
		Name:   libpf.Intern(name),
	}
	cd.files[name] = cf
	return cf
}

// FileMappingHeader64 is the header for CORE/NT_FILE note.
type FileMappingHeader64 struct {
	Entries  uint64
	PageSize uint64
}

// FileMappingEntry64 is the per-mapping data header in CORE/NT_FILE note.
type FileMappingEntry64 struct {
	Start, End, FileOffset uint64
}

// parseMappings processes a CORE/NT_FILE note with the description of memory mappings.
func (cd *CoredumpProcess) parseMappings(desc []byte,
	vaddrToMappings map[uint64]vaddrMappings) error {
	hdrSize := uint64(unsafe.Sizeof(FileMappingHeader64{}))
	entrySize := uint64(unsafe.Sizeof(FileMappingEntry64{}))

	if uint64(len(desc)) < hdrSize {
		return errors.New("too small NT_FILE section")
	}
	hdr := (*FileMappingHeader64)(unsafe.Pointer(&desc[0]))
	offs := hdrSize + hdr.Entries*entrySize
	// Check that we have at least data for the headers, and a zero terminator
	// byte for each of the per-entry filenames.
	if uint64(len(desc)) < offs+hdr.Entries {
		return errors.New("too small NT_FILE section")
	}
	strs := desc[offs:]
	for i := uint64(0); i < hdr.Entries; i++ {
		entry := (*FileMappingEntry64)(unsafe.Pointer(&desc[hdrSize+i*entrySize]))
		fnlen := bytes.IndexByte(strs, 0)
		if fnlen < 0 {
			return fmt.Errorf("corrupt NT_FILE: no filename #%d", i+1)
		}

		path := trimMappingPath(string(strs[:fnlen]))
		cf := cd.getFile(path)

		// In some cases, more than one entry with FO == 0 can exist. This occurs if the first
		// section is smaller than a memory page. It is then mapped again as a prefix for the next
		// section, presumably to allow the kernel to keep things on-demand paged. We thus pick the
		// smallest `Start`.
		if entry.FileOffset == 0 && (cf.Base == 0 || entry.Start < cf.Base) {
			cf.Base = entry.Start
		}

		if m, ok := vaddrToMappings[entry.Start]; ok {
			cm := CoredumpMapping{
				Prog:       m.prog,
				File:       cf,
				FileOffset: entry.FileOffset * hdr.PageSize,
			}
			cf.Mappings = append(cf.Mappings, cm)

			mapping := &cd.mappings[m.mappingIndex]
			mapping.Path = cf.Name
			mapping.FileOffset = entry.FileOffset * hdr.PageSize
			// Synthesize non-zero device and inode indicating this is a filebacked mapping.
			mapping.Device = 1
			mapping.Inode = cf.inode
		}
		strs = strs[fnlen+1:]
	}
	return nil
}

// parseAuxVector processes a CORE/NT_AUXV note.
func (cd *CoredumpProcess) parseAuxVector(desc []byte, vaddrToMappings map[uint64]vaddrMappings) {
	for i := 0; i+16 <= len(desc); i += 16 {
		value := binary.LittleEndian.Uint64(desc[i+8:])
		switch binary.LittleEndian.Uint64(desc[i:]) {
		case AT_SYSINFO_EHDR:
			m, ok := vaddrToMappings[value]
			if !ok {
				continue
			}

			vm := &cd.mappings[m.mappingIndex]
			vm.Inode = vdsoInode
			vm.Path = VdsoPathName

			cf := cd.getFile(vm.Path.String())
			cm := CoredumpMapping{
				Prog: m.prog,
				File: cf,
			}
			cf.Mappings = append(cf.Mappings, cm)

		case AT_PHDR:
			cd.execPhdrPtr = libpf.Address(value)
		}
	}
}

// PrpsInfo64 is the 64-bit NT_PRPSINFO note header.
type PrpsInfo64 struct {
	State  uint8
	Sname  uint8
	Zombie uint8
	Nice   uint8
	Gap    uint32
	Flags  uint64
	UID    uint32
	GID    uint32
	PID    uint32
	PPID   uint32
	PGRP   uint32
	SID    uint32
	FName  [16]byte
	Args   [80]byte
}

// parseProcessInfo processes a CORE/NT_PRPSINFO note.
func (cd *CoredumpProcess) parseProcessInfo(desc []byte) error {
	if len(desc) == int(unsafe.Sizeof(PrpsInfo64{})) {
		info := (*PrpsInfo64)(unsafe.Pointer(&desc[0]))
		cd.pid = libpf.PID(info.PID)
		cd.fname = string(info.FName[:])
		return nil
	}
	return fmt.Errorf("unsupported NT_PRPSINFO size: %d", len(desc))
}

// parseProcessStatus processes a CORE/NT_PRSTATUS note.
func (cd *CoredumpProcess) parseProcessStatus(desc []byte) error {
	// The corresponding struct definition can be found here:
	// https://github.com/torvalds/linux/blob/49d766f3a0e4/include/linux/elfcore.h#L48
	//
	// This code just extracts the few bits we are interested in. Because the
	// structure varies depending on platform, and we don't want the ELF parser
	// to only be able to decode the structure for the host architecture, we
	// manually hardcode the struct offsets for each relevant platform instead
	// of e.g. using CGO to cast it into a pointer.
	//
	// The offsets were calculated by running `pahole elf_prstatus` on a machine
	// with the corresponding architecture and then pasting the inferred struct
	// size and field offsets.

	var sizeof, regStart, regEnd int
	switch cd.Machine {
	case elf.EM_X86_64:
		sizeof = 336
		regStart = 112
		regEnd = 328
	case elf.EM_AARCH64:
		sizeof = 392
		regStart = 112
		regEnd = 384
	default:
		return fmt.Errorf("unsupported machine: %v", cd.Machine)
	}

	if len(desc) != sizeof {
		return fmt.Errorf("unsupported NT_PRSTATUS size: %d", len(desc))
	}

	ts := ThreadInfo{
		LWP:    binary.LittleEndian.Uint32(desc[32:]),
		GPRegs: desc[regStart:regEnd],
	}
	if cd.Machine == elf.EM_X86_64 {
		// Coredump GPRegs on x86_64 is actually "struct user_regs_struct" with
		// "struct pt_regs" followed by the segment register data. The fs_base
		// is the segment register at index 21.
		// See: "struct user_regs_struct" in linux/arch/x86/include/asm/user_64.h for the layout.
		ts.TPBase = binary.LittleEndian.Uint64(ts.GPRegs[21*8:])
	}
	cd.threadInfo = append(cd.threadInfo, ts)

	return nil
}

// parseArmPacMask parses the ARM64 specific section containing the PAC masks.
func (cd *CoredumpProcess) parseArmPacMask(desc []byte) error {
	// https://github.com/torvalds/linux/blob/1d1df41c5a33/arch/arm64/include/uapi/asm/ptrace.h#L250

	if len(desc) != 16 {
		return fmt.Errorf("unexpected aarch pauth section size %d, expected 16", len(desc))
	}

	cd.machineData.DataPACMask = binary.LittleEndian.Uint64(desc[0:8])
	cd.machineData.CodePACMask = binary.LittleEndian.Uint64(desc[8:16])

	return nil
}

// parseArmTLS parses the ARM specific section containing the TLS base address.
func (cd *CoredumpProcess) parseArmTLS(desc []byte) error {
	if len(desc) < 8 {
		return fmt.Errorf("unexpected aarch tls section size %d, expected at least 8", len(desc))
	}

	numThreads := len(cd.threadInfo)
	if numThreads == 0 {
		return errors.New("unexpected aarch tls section before NT_PRSTATUS")
	}

	// The TLS notes are interleaved between NT_PRSTATUS notes, so
	// this fixes up the TPBase of latest seen thread.
	cd.threadInfo[numThreads-1].TPBase = binary.LittleEndian.Uint64(desc)

	return nil
}

// ReadAt reads a file inside a core dump from given file offset.
func (cf *CoredumpFile) ReadAt(p []byte, addr int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	addrEnd := uint64(addr) + uint64(len(p))
	for _, cm := range cf.Mappings {
		if uint64(addr) >= cm.FileOffset &&
			addrEnd <= cm.FileOffset+cm.Prog.Filesz {
			return cm.Prog.ReadAt(p, addr-int64(cm.FileOffset))
		}
	}
	return 0, fmt.Errorf("core does not have data for file '%s' at 0x%x",
		cf.Name, addr)
}

// OpenELF opens the CoredumpFile as an ELF.
//
// The returned `pfelf.File` is borrowing the coredump file. Closing it will not close the
// underlying CoredumpFile.
func (cf *CoredumpFile) OpenELF() (*pfelf.File, error) {
	return pfelf.NewFile(cf, cf.Base, cf.parent.hasMusl)
}
