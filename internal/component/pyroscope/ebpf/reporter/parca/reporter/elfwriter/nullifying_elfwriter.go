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
	"fmt"
	"io"
)

// NullifyingWriter is a wrapper around another Writer that nullifies all the sections
// except the whitelisted ones.
type NullifyingWriter struct {
	Writer
	src io.ReaderAt

	progPredicates    []func(*elf.Prog) bool
	sectionPredicates []func(*elf.Section) bool
}

// NewNullifyingWriter creates a new NullifyingWriter.
func NewNullifyingWriter(dst io.WriteSeeker, src io.ReaderAt, opts ...Option) (*NullifyingWriter, error) {
	f, err := elf.NewFile(src)
	if err != nil {
		return nil, fmt.Errorf("error reading ELF file: %w", err)
	}
	defer f.Close()

	w, err := newWriter(dst, &f.FileHeader, newNullifyingWriterSectionReader(src), opts...)
	if err != nil {
		return nil, err
	}
	w.progs = f.Progs
	w.sections = f.Sections

	return &NullifyingWriter{
		Writer: *w,
		src:    src,
	}, nil
}

// FilterPrograms filters out programs from the source.
func (w *NullifyingWriter) FilterPrograms(predicates ...func(*elf.Prog) bool) {
	w.progPredicates = append(w.progPredicates, predicates...)
}

// KeepSections keeps only the sections that match the predicates.
// If no predicates are given, all sections are nullified.
func (w *NullifyingWriter) KeepSections(predicates ...func(*elf.Section) bool) {
	w.sectionPredicates = append(w.sectionPredicates, predicates...)
}

func newNullifyingWriterSectionReader(src io.ReaderAt) sectionReaderProviderFn {
	return func(sec elf.Section) (io.Reader, error) {
		if sec.Type == elf.SHT_NOBITS {
			return nil, nil
		}
		return io.NewSectionReader(src, int64(sec.Offset), int64(sec.FileSize)), nil
	}
}

func (w *NullifyingWriter) Flush() error {
	if len(w.progPredicates) > 0 {
		newProgs := []*elf.Prog{}
		for _, prog := range w.progs {
			if match(prog, w.progPredicates...) {
				newProgs = append(newProgs, prog)
			}
		}
		w.progs = newProgs
	}

	for _, sec := range w.sections {
		if match(sec, w.sectionPredicates...) ||
			sec.Type == elf.SHT_NOBITS || sec.Type == elf.SHT_NULL || isSectionStringTable(sec) {
			continue
		}
		sec.Type = elf.SHT_NOBITS
	}

	return w.Writer.Flush()
}
