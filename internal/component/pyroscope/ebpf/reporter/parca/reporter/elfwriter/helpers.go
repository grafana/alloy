// Copyright 2022-2024 The Parca Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package elfwriter

import (
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zlib"
)

type zeroReader struct{}

func (*zeroReader) ReadAt(p []byte, off int64) (_ int, _ error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

type countingWriter struct {
	w       io.Writer
	written int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.written += int64(n)
	return n, err
}

func isCompressed(sec *elf.Section) bool {
	return sec.Type == elf.SHT_PROGBITS &&
		(sec.Flags&elf.SHF_COMPRESSED != 0 || strings.HasPrefix(sec.Name, ".zdebug_"))
}

type compressionHeader struct {
	byteOrder  binary.ByteOrder
	class      elf.Class
	headerSize int

	Type      uint32
	Size      uint64
	Addralign uint64
}

func NewCompressionHeaderFromSource(fhdr *elf.FileHeader, src io.ReaderAt, offset int64) (*compressionHeader, error) {
	hdr := &compressionHeader{}

	switch fhdr.Class {
	case elf.ELFCLASS32:
		ch := new(elf.Chdr32)
		hdr.headerSize = binary.Size(ch)
		sr := io.NewSectionReader(src, offset, int64(hdr.headerSize))
		if err := binary.Read(sr, fhdr.ByteOrder, ch); err != nil {
			return nil, err
		}
		hdr.class = elf.ELFCLASS32
		hdr.Type = ch.Type
		hdr.Size = uint64(ch.Size)
		hdr.Addralign = uint64(ch.Addralign)
		hdr.byteOrder = fhdr.ByteOrder
	case elf.ELFCLASS64:
		ch := new(elf.Chdr64)
		hdr.headerSize = binary.Size(ch)
		sr := io.NewSectionReader(src, offset, int64(hdr.headerSize))
		if err := binary.Read(sr, fhdr.ByteOrder, ch); err != nil {
			return nil, err
		}
		hdr.class = elf.ELFCLASS64
		hdr.Type = ch.Type
		hdr.Size = ch.Size
		hdr.Addralign = ch.Addralign
		hdr.byteOrder = fhdr.ByteOrder
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		return nil, fmt.Errorf("unknown ELF class: %v", fhdr.Class)
	}

	if elf.CompressionType(hdr.Type) != elf.COMPRESS_ZLIB {
		// TODO(kakkoyun): COMPRESS_ZSTD
		// https://github.com/golang/go/issues/55107
		return nil, errors.New("section should be zlib compressed, we are reading from the wrong offset or debug data is corrupt")
	}

	return hdr, nil
}

func (hdr compressionHeader) WriteTo(w io.Writer) (int64, error) {
	var written int
	switch hdr.class {
	case elf.ELFCLASS32:
		ch := new(elf.Chdr32)
		ch.Type = uint32(elf.COMPRESS_ZLIB)
		ch.Size = uint32(hdr.Size)
		ch.Addralign = uint32(hdr.Addralign)
		if err := binary.Write(w, hdr.byteOrder, ch); err != nil {
			return 0, err
		}
		written = binary.Size(ch) // headerSize
	case elf.ELFCLASS64:
		ch := new(elf.Chdr64)
		ch.Type = uint32(elf.COMPRESS_ZLIB)
		ch.Size = hdr.Size
		ch.Addralign = hdr.Addralign
		if err := binary.Write(w, hdr.byteOrder, ch); err != nil {
			return 0, err
		}
		written = binary.Size(ch) // headerSize
	case elf.ELFCLASSNONE:
		fallthrough
	default:
		return 0, fmt.Errorf("unknown ELF class: %v", hdr.class)
	}

	return int64(written), nil
}

func copyCompressed(w io.Writer, r io.Reader) (int64, error) {
	if r == nil {
		return 0, errors.New("reader is nil")
	}

	pr, pw := io.Pipe()

	// write in writer end of pipe.
	var wErr error
	go func() {
		defer pw.Close()
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if ok {
					wErr = fmt.Errorf("panic occurred: %w", err)
				}
			}
		}()
		_, wErr = io.Copy(pw, r)
	}()

	// read from reader end of pipe.
	defer pr.Close()

	cw := &countingWriter{w: w}
	zw := zlib.NewWriter(cw)
	_, err := io.Copy(zw, pr)
	if err != nil {
		zw.Close()
		return 0, err
	}
	zw.Close()

	if wErr != nil {
		return 0, wErr
	}
	return cw.written, nil
}

func isDWARF(s *elf.Section) bool {
	return strings.HasPrefix(s.Name, ".debug_") ||
		strings.HasPrefix(s.Name, ".zdebug_") ||
		strings.HasPrefix(s.Name, "__debug_") // macos
}

func isSymbolTable(s *elf.Section) bool {
	return s.Type == elf.SHT_SYMTAB || s.Type == elf.SHT_DYNSYM ||
		s.Type == elf.SHT_STRTAB ||
		s.Name == ".symtab" ||
		s.Name == ".dynsym" ||
		s.Name == ".strtab" ||
		s.Name == ".dynstr"
}

func isGoSymbolTable(s *elf.Section) bool {
	return s.Name == ".gosymtab" ||
		s.Name == ".gopclntab" ||
		s.Name == ".go.buildinfo" ||
		s.Name == ".data.rel.ro.gosymtab" ||
		s.Name == ".data.rel.ro.gopclntab"
}

func isPltSymbolTable(s *elf.Section) bool {
	return s.Type == elf.SHT_RELA || s.Type == elf.SHT_REL || // nolint:misspell
		// Redundant
		s.Name == ".plt" ||
		s.Name == ".plt.got" ||
		s.Name == ".rela.plt" ||
		s.Name == ".rela.dyn" // nolint:goconst
}

func match[T *elf.Prog | *elf.Section | *elf.SectionHeader](elem T, predicates ...func(T) bool) bool {
	for _, pred := range predicates {
		if pred(elem) {
			return true
		}
	}
	return false
}
