// Copyright (C) 2025 go-delve, parca-agent 
// 
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// 
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
// 
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE 
// SOFTWARE.
// -----------------------------------------------------------------------------
// Package elfwriter is a package to write ELF files without having their entire
// contents in memory at any one time.
//
// Original work started from https://github.com/go-delve/delve/blob/master/pkg/elfwriter/writer.go
// and additional functionality added on top.
//
// This package does not provide completeness guarantees. Some of the missing features:
// - Consistency and soundness of relocations
// - Consistency and preservation of linked sections (when target removed (sh_link)) - partially supported
// - Consistency and existence of overlapping segments when a section removed (offset, range check)
package elfwriter

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sys/unix"
)

const sectionHeaderStrTable = ".shstrtab"

// http://www.sco.com/developers/gabi/2003-12-17/ch4.sheader.html#special_sections
// - Figure 4-12
// The list is incomplete list.
var specialSectionLinks = map[string]string{
	// Source - Target
	".symtab": ".strtab",
	".dynsym": ".dynstr",
}

// TODO(kakkoyun): Remove FilteringWriter and remove this interface and pattern.
type sectionReaderProvider interface {
	sectionReader(section elf.Section) (io.Reader, error)
}

type sectionReaderProviderFn func(section elf.Section) (io.Reader, error)

func (fn sectionReaderProviderFn) sectionReader(section elf.Section) (io.Reader, error) {
	return fn(section)
}

// Writer writes ELF files.
type Writer struct {
	dst                   io.WriteSeeker
	fhdr                  *elf.FileHeader
	compressDWARFSections bool

	srProvider sectionReaderProvider

	// Program headers to write in the underlying io.WriteSeeker.
	progs []*elf.Prog
	// sections to write in the underlying io.WriteSeeker.
	sections []*elf.Section
	// sections to write in the underlying io.WriteSeeker without data.
	sectionHeaders []elf.SectionHeader
	// additional notes to write in the underlying io.WriteSeeker.
	additionalNotes []Note

	// Source - Target
	sectionLinks map[string]string

	err error

	seekProgHeader       int64 // position of phoff
	seekProgNum          int64 // position of phnum
	seekSectionHeader    int64 // position of shoff
	seekSectionNum       int64 // position of shnun
	seekSectionStringIdx int64 // position of shstrndx
	seekSectionEntrySize int64

	// For validation.
	ehsize, phentsize, shentsize uint16
	shnum, shoff, shstrndx       int

	shStrIdx map[string]int
}

// newWriter creates a new Writer.
func newWriter(w io.WriteSeeker, fhdr *elf.FileHeader, srp sectionReaderProvider, opts ...Option) (*Writer, error) {
	if fhdr.ByteOrder == nil {
		return nil, errors.New("byte order has to be specified")
	}

	switch fhdr.Class {
	case elf.ELFCLASS32:
	case elf.ELFCLASS64:
	// Ok
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		return nil, errors.New("unknown ELF class")
	}

	sectionLinks := make(map[string]string)
	for k, v := range specialSectionLinks {
		sectionLinks[k] = v
	}
	wrt := &Writer{
		dst:        w,
		fhdr:       fhdr,
		srProvider: srp,

		shStrIdx:     make(map[string]int),
		sectionLinks: sectionLinks,
	}
	for _, opt := range opts {
		opt(wrt)
	}
	return wrt, nil
}

type Note struct {
	Type elf.NType
	Name string
	Data []byte
}

// Flush writes any buffered data to the underlying io.WriterSeeker.
func (w *Writer) Flush() error {
	// +-------------------------------+
	// | ELF File Header               |
	// +-------------------------------+
	// | Program Header for segment #1 |
	// +-------------------------------+
	// | Program Header for segment #2 |
	// +-------------------------------+
	// | ...                           |
	// +-------------------------------+
	// | Contents (Byte Stream)        |
	// | ...                           |
	// +-------------------------------+
	// | Section Header for section #1 |
	// +-------------------------------+
	// | Section Header for section #2 |
	// +-------------------------------+
	// | ...                           |
	// +-------------------------------+
	// | ".shstrtab" section           |
	// +-------------------------------+
	// | ".symtab"   section           |
	// +-------------------------------+
	// | ".strtab"   section           |
	// +-------------------------------+

	// 1. File Header
	// 2. Program Header Table
	// 3. sections
	// 4. Section Header Table
	w.writeFileHeader()
	if w.err != nil {
		return fmt.Errorf("failed to write file header: %w", w.err)
	}
	w.writeNotes()
	if w.err != nil {
		return fmt.Errorf("failed to write notes: %w", w.err)
	}
	w.writeSegments()
	if w.err != nil {
		return fmt.Errorf("failed to write segments: %w", w.err)
	}
	w.writeSections()
	if w.err != nil {
		return fmt.Errorf("failed to write sections: %w", w.err)
	}

	if w.shoff == 0 && w.shnum != 0 {
		return fmt.Errorf("invalid ELF shnum=%d for shoff=0", w.shnum)
	}
	if w.shnum > 0 && w.shstrndx >= w.shnum {
		return fmt.Errorf("invalid ELF shstrndx=%d", w.shstrndx)
	}
	return nil
}

// Reset discards any unflushed buffered data, clears any error, and resets data to write its output to dst.
func (w *Writer) Reset(ws io.WriteSeeker) {
	w.dst = ws
	w.err = nil

	w.progs = nil
	w.sections = nil
	w.sectionHeaders = nil
	w.additionalNotes = nil

	w.seekProgHeader = 0
	w.seekProgNum = 0
	w.seekSectionHeader = 0
	w.seekSectionNum = 0
	w.seekSectionStringIdx = 0
	w.seekSectionEntrySize = 0

	w.ehsize = 0
	w.phentsize = 0
	w.shentsize = 0

	w.shnum = 0
	w.shoff = 0
	w.shstrndx = 0
	w.shStrIdx = make(map[string]int)
}

// AddNotes adds additional notes to write to in the underlying io.WriterSeeker.
func (w *Writer) AddNotes(additionalNotes ...Note) {
	w.additionalNotes = append(w.additionalNotes, additionalNotes...)
}

// writeFileHeader writes the initial file header using given information.
func (w *Writer) writeFileHeader() {
	fhdr := w.fhdr

	switch fhdr.Class {
	case elf.ELFCLASS32:
		w.ehsize = 52
		w.phentsize = 32
		w.shentsize = 40
	case elf.ELFCLASS64:
		w.ehsize = 64
		w.phentsize = 56
		w.shentsize = 64
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		w.err = fmt.Errorf("unknown ELF class: %v", w.fhdr.Class)
		return
	}

	// e_ident
	w.write([]byte{
		0x7f, 'E', 'L', 'F', // Magic number
		byte(fhdr.Class),
		byte(fhdr.Data),
		byte(fhdr.Version),
		byte(fhdr.OSABI),
		fhdr.ABIVersion,
		0, 0, 0, 0, 0, 0, 0, // Padding
	})

	switch fhdr.Class {
	case elf.ELFCLASS32:
		// type Header32 struct {
		// 	Ident     [EI_NIDENT]byte /* File identification. */
		// 	Type      uint16          /* File type. */
		// 	Machine   uint16          /* Machine architecture. */
		// 	Version   uint32          /* ELF format version. */
		// 	Entry     uint32          /* Entry point. */
		// 	Phoff     uint32          /* Program header file offset. */
		// 	Shoff     uint32          /* Section header file offset. */
		// 	Flags     uint32          /* Architecture-specific flags. */
		// 	Ehsize    uint16          /* Size of ELF header in bytes. */
		// 	Phentsize uint16          /* Size of program header entry. */
		// 	Phnum     uint16          /* Number of program header entries. */
		// 	Shentsize uint16          /* Size of section header entry. */
		// 	Shnum     uint16          /* Number of section header entries. */
		// 	Shstrndx  uint16          /* Section name strings section. */
		// }
		w.u16(uint16(fhdr.Type))    // e_type
		w.u16(uint16(fhdr.Machine)) // e_machine
		w.u32(uint32(fhdr.Version)) // e_version
		w.u32(uint32(0))            // e_entry
		w.seekProgHeader = w.here()
		w.u32(uint32(0)) // e_phoff
		w.seekSectionHeader = w.here()
		w.u32(uint32(0))   // e_shoff
		w.u32(uint32(0))   // e_flags
		w.u16(w.ehsize)    // e_ehsize
		w.u16(w.phentsize) // e_phentsize
		w.seekProgNum = w.here()
		w.u16(uint16(0)) // e_phnum
		w.seekSectionEntrySize = w.here()
		w.u16(w.shentsize) // e_shentsize
		w.seekSectionNum = w.here()
		w.u16(uint16(0)) // e_shnum
		w.seekSectionStringIdx = w.here()
		w.u16(uint16(elf.SHN_UNDEF)) // e_shstrndx
	case elf.ELFCLASS64:
		// type Header64 struct {
		// 	Ident     [EI_NIDENT]byte /* File identification. */
		// 	Type      uint16          /* File type. */
		// 	Machine   uint16          /* Machine architecture. */
		// 	Version   uint32          /* ELF format version. */
		// 	Entry     uint64          /* Entry point. */
		// 	Phoff     uint64          /* Program header file offset. */
		// 	Shoff     uint64          /* Section header file offset. */
		// 	Flags     uint32          /* Architecture-specific flags. */
		// 	Ehsize    uint16          /* Size of ELF header in bytes. */
		// 	Phentsize uint16          /* Size of program header entry. */
		// 	Phnum     uint16          /* Number of program header entries. */
		// 	Shentsize uint16          /* Size of section header entry. */
		// 	Shnum     uint16          /* Number of section header entries. */
		// 	Shstrndx  uint16          /* Section name strings section. */
		// }
		w.u16(uint16(fhdr.Type))    // e_type
		w.u16(uint16(fhdr.Machine)) // e_machine
		w.u32(uint32(fhdr.Version)) // e_version
		w.u64(uint64(0))            // e_entry
		w.seekProgHeader = w.here()
		w.u64(uint64(0)) // e_phoff
		w.seekSectionHeader = w.here()
		w.u64(uint64(0))   // e_shoff
		w.u32(uint32(0))   // e_flags
		w.u16(w.ehsize)    // e_ehsize
		w.u16(w.phentsize) // e_phentsize
		w.seekProgNum = w.here()
		w.u16(uint16(0)) // e_phnum
		w.seekSectionEntrySize = w.here()
		w.u16(w.shentsize) // e_shentsize
		w.seekSectionNum = w.here()
		w.u16(uint16(0)) // e_shnum
		w.seekSectionStringIdx = w.here()
		w.u16(uint16(elf.SHN_UNDEF)) // e_shstrndx
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		w.err = fmt.Errorf("unknown ELF class: %v", w.fhdr.Class)
	}

	// Sanity check, size of file header should be the same as ehsize
	if sz, _ := w.dst.Seek(0, io.SeekCurrent); sz != int64(w.ehsize) {
		w.err = errors.New("internal error, ELF header size")
	}
}

// writeNotes writes notes to the current location, and adds a ProgHeader describing the notes.
func (w *Writer) writeNotes() {
	// http://www.sco.com/developers/gabi/latest/ch5.pheader.html#note_section
	if len(w.additionalNotes) == 0 {
		return
	}

	notes := w.additionalNotes
	h := &elf.ProgHeader{
		Type: elf.PT_NOTE,
	}

	write32 := func(note *Note) {
		// Note header in a PT_NOTE section
		// typedef struct elf32_note {
		//   Elf32_Word	n_namesz;	/* Name size */
		//   Elf32_Word	n_descsz;	/* Content size */
		//   Elf32_Word	n_type;		/* Content type */
		// } Elf32_Nhdr;
		//
		align := uint64(4)
		h.Align = align
		w.align(int64(align))
		if h.Off == 0 {
			h.Off = uint64(w.here())
		}
		w.u32(uint32(len(note.Name))) // n_namesz
		w.u32(uint32(len(note.Data))) // n_descsz
		w.u32(uint32(note.Type))      // n_type
		w.write([]byte(note.Name))
		w.align(int64(align))
		w.write(note.Data)
	}

	write64 := func(note *Note) {
		// TODO: This might be incorrect. (At least for Go).
		// - https://github.com/google/pprof/blob/d04f2422c8a17569c14e84da0fae252d9529826b/internal/elfexec/elfexec.go#L56-L58

		// Note header in a PT_NOTE section
		// typedef struct elf64_note {
		//   Elf64_Word n_namesz;	/* Name size */
		//   Elf64_Word n_descsz;	/* Content size */
		//   Elf64_Word n_type;	/* Content type */
		// } Elf64_Nhdr;
		align := uint64(8)
		h.Align = align
		w.align(int64(align))
		if h.Off == 0 {
			h.Off = uint64(w.here())
		}
		w.u64(uint64(len(note.Name))) // n_namesz
		w.u64(uint64(len(note.Data))) // n_descsz
		w.u64(uint64(note.Type))      // n_type
		w.write([]byte(note.Name))
		w.align(int64(align))
		w.write(note.Data)
	}

	var write func(note *Note)
	switch w.fhdr.Class {
	case elf.ELFCLASS32:
		write = write32
	case elf.ELFCLASS64:
		write = write64
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		w.err = fmt.Errorf("unknown ELF class: %v", w.fhdr.Class)
	}

	for i := range notes {
		write(&notes[i])
	}
	h.Filesz = uint64(w.here()) - h.Off
	w.progs = append(w.progs, &elf.Prog{ProgHeader: *h})
}

// writeSegments writes the program headers at the current location
// and patches the file header accordingly.
func (w *Writer) writeSegments() {
	if len(w.progs) == 0 {
		return
	}

	// http://www.sco.com/developers/gabi/latest/ch5.pheader.html
	phoff := w.here()
	phnum := uint64(len(w.progs))

	// Patch file header.
	w.seek(w.seekProgHeader, io.SeekStart)
	w.u64(uint64(phoff))
	w.seek(w.seekProgNum, io.SeekStart)
	w.u64(phnum) // e_phnum
	w.seek(0, io.SeekEnd)

	writePH32 := func(prog *elf.Prog) {
		// ELF32 Program header.
		// type Prog32 struct {
		// 	Type   uint32 /* Entry type. */
		// 	Off    uint32 /* File offset of contents. */
		// 	Vaddr  uint32 /* Virtual address in memory image. */
		// 	Paddr  uint32 /* Physical address (not used). */
		// 	Filesz uint32 /* Size of contents in file. */
		// 	Memsz  uint32 /* Size of contents in memory. */
		// 	Flags  uint32 /* Access permission flags. */
		// 	Align  uint32 /* Alignment in memory and file. */
		// }
		w.u32(uint32(prog.Type))
		w.u32(uint32(prog.Off))
		w.u32(uint32(prog.Vaddr))
		w.u32(uint32(prog.Paddr))
		w.u32(uint32(prog.Filesz))
		w.u32(uint32(prog.Memsz))
		w.u32(uint32(prog.Flags))
		w.u32(uint32(prog.Align))
	}

	writePH64 := func(prog *elf.Prog) {
		// ELF64 Program header.
		// type Prog64 struct {
		// 	Type   uint32 /* Entry type. */
		// 	Flags  uint32 /* Access permission flags. */
		// 	Off    uint64 /* File offset of contents. */
		// 	Vaddr  uint64 /* Virtual address in memory image. */
		// 	Paddr  uint64 /* Physical address (not used). */
		// 	Filesz uint64 /* Size of contents in file. */
		// 	Memsz  uint64 /* Size of contents in memory. */
		// 	Align  uint64 /* Alignment in memory and file. */
		// }
		w.u32(uint32(prog.Type))
		w.u32(uint32(prog.Flags))
		w.u64(prog.Off)
		w.u64(prog.Vaddr)
		w.u64(prog.Paddr)
		w.u64(prog.Filesz)
		w.u64(prog.Memsz)
		w.u64(prog.Align)
	}

	var writeProgramHeader func(prog *elf.Prog)
	switch w.fhdr.Class {
	case elf.ELFCLASS32:
		writeProgramHeader = writePH32
	case elf.ELFCLASS64:
		writeProgramHeader = writePH64
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		w.err = fmt.Errorf("unknown ELF class: %v", w.fhdr.Class)
	}

	for _, prog := range w.progs {
		// Write program header to program header table.
		writeProgramHeader(prog)
	}
}

// writeSections writes the sections at the current location
// and patches the file header accordingly.
func (w *Writer) writeSections() {
	if len(w.sections) == 0 {
		return
	}
	// http://www.sco.com/developers/gabi/2003-12-17/ch4.sheader.html
	// 			   +-------------------+
	// 			   | ELF header        |---+  e_shoff
	// 			   +-------------------+   |
	// 			   | Section 0         |<-----+
	// 			   +-------------------+   |  | sh_offset
	// 			   | Section 1         |<--|-----+
	// 			   +-------------------+   |  |  |
	// 			   | Section 2         |<--|--|--|--+
	// +---------> +-------------------+   |  |  |  |
	// |           |                   |<--+  |  |  |
	// | Section   | Section header 0  |      |  |  |
	// |           |                   |<-----+  |  |
	// | Header    +-------------------+         |  |
	// |           | Section header 1  |<--------+  |
	// | Table     +-------------------+            |
	// |           | Section header 2  |------------+ sh_offset
	// +---------> +-------------------+

	// Shallow copy the section for further editing.
	copySection := func(s *elf.Section) *elf.Section {
		clone := new(elf.Section)
		*clone = *s
		return clone
	}

	// sections that will end up in the output.
	stw := make([]*elf.Section, 0, len(w.sections)+2)

	// Build section header string table.
	shstrtab := new(elf.Section)
	shstrtab.Name = sectionHeaderStrTable
	shstrtab.Type = elf.SHT_STRTAB
	shstrtab.Addralign = 1

	sectionNameIdx := make(map[string]int)
	i := 0
	for _, sec := range w.sections {
		if i == 0 {
			if sec.Type == elf.SHT_NULL {
				stw = append(stw, copySection(sec))
				i++
				continue
			}
			s := new(elf.Section)
			s.Type = elf.SHT_NULL
			stw = append(stw, s)
			i++
		}
		// There can be more than one section with the same name.
		// The relevant section header string table is the one denoted by the elf header's shstrndx field.
		// For our use case, we can skip the section header string table,
		// because it will be replaced by the one we are building.
		// http://www.sco.com/developers/gabi/latest/ch4.sheader.html#shstrndx
		if isSectionStringTable(sec) {
			continue
		}
		stw = append(stw, copySection(sec))
		sectionNameIdx[sec.Name] = i
		i++
	}
	for _, sh := range w.sectionHeaders {
		// NOTICE: elf.Section.Open suppose to return a zero reader if the section type is no bits.
		// However it doesn't respect SHT_NOBITS, so better to set the size to 0.
		sh.Type = elf.SHT_NOBITS
		sh.Size = 0
		sh.FileSize = 0
		i, ok := sectionNameIdx[sh.Name]
		if ok {
			stw[i] = &elf.Section{SectionHeader: sh}
		} else {
			stw = append(stw, &elf.Section{SectionHeader: sh})
		}
	}
	if w.shstrndx == 0 {
		stw = append(stw, shstrtab)
		w.shstrndx = len(stw) - 1
		sectionNameIdx[sectionHeaderStrTable] = w.shstrndx
	}

	shnum := len(stw)
	w.shnum = shnum

	names := make([]string, shnum)
	for i, sec := range stw {
		if sec.Name != "" {
			names[i] = sec.Name
		}
	}

	// Start writing actual data for sections.
	for i, sec := range stw {
		var (
			written     int64
			entryOffset = w.here()
		)
		// The section header string section is reserved for section header string table.
		if i == w.shstrndx {
			w.writeStringTable(names)
			sec.Size = uint64(w.here() - entryOffset)
		} else {
			if sec.Type == elf.SHT_NULL {
				continue
			}

			sr, err := w.srProvider.sectionReader(*sec)
			if err != nil && w.err == nil {
				w.err = err
			}

			if sr != nil {
				if w.compressDWARFSections && isDWARF(sec) && !isCompressed(sec) {
					// Compress DWARF sections.
					cw := &countingWriter{w: io.Discard}
					tr := io.TeeReader(sr, cw)

					buf := bytes.NewBuffer(nil)
					bufWritten, err := copyCompressed(buf, tr)
					if err != nil && w.err == nil {
						w.err = err
					}

					ch := compressionHeader{
						byteOrder: w.fhdr.ByteOrder,
						class:     w.fhdr.Class,
						Type:      uint32(elf.COMPRESS_ZLIB),
						Size:      uint64(cw.written), // read bytes from the section reader.
						Addralign: sec.SectionHeader.Addralign,
					}
					hdrWritten, err := ch.WriteTo(w.dst)
					if err != nil && w.err == nil {
						w.err = err
					}

					dataWritten, err := buf.WriteTo(w.dst)
					if err != nil && w.err == nil {
						w.err = err
					}

					if bufWritten != dataWritten {
						w.err = fmt.Errorf("section %s: expected %d bytes, wrote %d bytes", sec.Name, bufWritten, dataWritten)
					}

					written = hdrWritten + dataWritten
					sec.Flags |= elf.SHF_COMPRESSED
				} else {
					// Write as is.
					written, err = io.Copy(w.dst, sr)
					if err != nil && w.err == nil {
						w.err = err
					}
				}
			}

			diff := (w.here() - entryOffset)
			if written != diff {
				w.err = fmt.Errorf("section %s: expected %d bytes, wrote %d bytes", sec.Name, written, diff)
			}
		}

		sec.Offset = uint64(entryOffset)
		sec.FileSize = uint64(w.here() - entryOffset)
		if w.err != nil {
			// Early exit if there is an error.
			return
		}
	}

	// Start writing the section header table.
	shoff := w.here()
	w.shoff = int(shoff)
	// First, patch file header.
	w.seek(w.seekSectionHeader, io.SeekStart)
	w.u64(uint64(shoff))
	w.seek(w.seekSectionNum, io.SeekStart)
	w.u16(uint16(shnum)) // e_shnum
	w.seek(w.seekSectionStringIdx, io.SeekStart)
	w.u16(uint16(w.shstrndx))
	w.seek(w.seekSectionEntrySize, io.SeekStart)
	w.u16(w.shentsize) // e_shentsize
	w.seek(0, io.SeekEnd)

	writeLink := func(sec *elf.Section) {
		if sec.Link > 0 {
			target, ok := w.sectionLinks[sec.Name]
			if ok {
				w.u32(uint32(sectionNameIdx[target]))
			} else {
				w.u32(uint32(0))
			}
		} else {
			w.u32(uint32(0))
		}
	}
	writeSH32 := func(shstrndx int, sec *elf.Section) {
		// ELF32 Section header.
		// type Section32 struct {
		// 	Name      uint32 /* Section name (index into the section header string table). */
		// 	Type      uint32 /* Section type. */
		// 	Flags     uint32 /* Section flags. */
		// 	Addr      uint32 /* Address in memory image. */
		// 	Off       uint32 /* Offset in file. */
		// 	Size      uint32 /* Size in bytes. */
		// 	Link      uint32 /* Index of a related section. */
		// 	Info      uint32 /* Depends on section type. */
		// 	Addralign uint32 /* Alignment in bytes. */
		// 	Entsize   uint32 /* Size of each entry in section. */
		// }
		w.u32(uint32(shstrndx))
		w.u32(uint32(sec.Type))
		w.u32(uint32(sec.Flags))
		w.u32(uint32(sec.Addr))
		w.u32(uint32(sec.Offset))
		w.u32(uint32(sec.FileSize))
		writeLink(sec)
		w.u32(sec.Info)
		w.u32(uint32(sec.Addralign))
		w.u32(uint32(sec.Entsize))
	}

	writeSH64 := func(shstrndx int, sec *elf.Section) {
		// ELF64 Section header.
		// type Section64 struct {
		// 	Name      uint32 /* Section name (index into the section header string table). */
		// 	Type      uint32 /* Section type. */
		// 	Flags     uint64 /* Section flags. */
		// 	Addr      uint64 /* Address in memory image. */
		// 	Off       uint64 /* Offset in file. */
		// 	Size      uint64 /* Size in bytes. */
		// 	Link      uint32 /* Index of a related section. */
		// 	Info      uint32 /* Depends on section type. */
		// 	Addralign uint64 /* Alignment in bytes. */
		// 	Entsize   uint64 /* Size of each entry in section. */
		// }
		w.u32(uint32(shstrndx))
		w.u32(uint32(sec.Type))
		w.u64(uint64(sec.Flags))
		w.u64(sec.Addr)
		w.u64(sec.Offset)
		w.u64(sec.FileSize)
		writeLink(sec)
		w.u32(sec.Info)
		w.u64(sec.Addralign)
		w.u64(sec.Entsize)
	}

	// shstrndx index of the entry in the section header string table.
	// 0 reserved for null string.
	var writeSectionHeader func(shstrndx int, sec *elf.Section)
	switch w.fhdr.Class {
	case elf.ELFCLASS32:
		writeSectionHeader = writeSH32
	case elf.ELFCLASS64:
		writeSectionHeader = writeSH64
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		w.err = fmt.Errorf("unknown ELF class: %v", w.fhdr.Class)
	}

	for _, sec := range stw {
		if sec.Name == "" {
			writeSectionHeader(0, sec)
			continue
		}
		writeSectionHeader(w.shStrIdx[sec.Name], sec)
	}
}

// here returns the current seek offset from the start of the file.
func (w *Writer) here() int64 {
	r, err := w.dst.Seek(0, io.SeekCurrent)
	if err != nil && w.err == nil {
		w.err = err
	}
	return r
}

// seek moves the cursor to the point calculated using offset and starting point.
func (w *Writer) seek(offset int64, whence int) {
	_, err := w.dst.Seek(offset, whence)
	if err != nil && w.err == nil {
		w.err = err
	}
}

// align writes as many padding bytes as needed to make the current file
// offset a multiple of align.
func (w *Writer) align(align int64) {
	off := w.here()
	alignOff := (off + (align - 1)) &^ (align - 1)
	if alignOff-off > 0 {
		w.write(make([]byte, alignOff-off))
	}
}

func (w *Writer) write(buf []byte) {
	_, err := w.dst.Write(buf)
	if err != nil && w.err == nil {
		w.err = err
	}
}

func (w *Writer) u16(n uint16) {
	err := binary.Write(w.dst, w.fhdr.ByteOrder, n)
	if err != nil && w.err == nil {
		w.err = err
	}
}

func (w *Writer) u32(n uint32) {
	err := binary.Write(w.dst, w.fhdr.ByteOrder, n)
	if err != nil && w.err == nil {
		w.err = err
	}
}

func (w *Writer) u64(n uint64) {
	err := binary.Write(w.dst, w.fhdr.ByteOrder, n)
	if err != nil && w.err == nil {
		w.err = err
	}
}

// writeStringTable writes given strings in string table format.
func (w *Writer) writeStringTable(strs []string) {
	// http://www.sco.com/developers/gabi/2003-12-17/ch4.strtab.html
	w.write([]byte{0})
	i := 1
	for _, s := range strs {
		if s == "" {
			continue
		}
		data, err := unix.ByteSliceFromString(s)
		if err != nil && w.err == nil {
			w.err = err
			break
		}
		w.shStrIdx[s] = i
		w.write(data)
		i += len(data)
	}
}

func isSectionStringTable(sec *elf.Section) bool {
	return sec.Type == elf.SHT_STRTAB && sec.Name == sectionHeaderStrTable
}

func newSectionReaderWithoutRawSource(fhdr *elf.FileHeader) sectionReaderProviderFn {
	return func(sec elf.Section) (io.Reader, error) {
		// Opens the header. If it is compressed, it will un-compress it.
		// If compressed, it will skip past the compression header [1] and
		// give a reader to the section itself.
		//
		// - [1] https://github.com/golang/go/blob/cd33b4089caf362203cd749ee1b3680b72a8c502/src/debug/elf/file.go#L132
		r := sec.Open()
		if sec.Type == elf.SHT_NOBITS {
			r = io.NewSectionReader(&zeroReader{}, 0, 0)
		}
		return r, nil
	}
}
