// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package pfelf implements functions for processing of ELF files and extracting data from
// them. This file provides a cacheable file offset to virtual address mapping.
package pfelf // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf"

import (
	"debug/elf"
	"os"
)

// addressMapperPHDR contains the Program Header fields we need to cache for mapping
// file offsets to virtual addresses.
type addressMapperPHDR struct {
	offset uint64
	vaddr  uint64
	filesz uint64
}

// AddressMapper contains minimal information about PHDRs needed for address mapping
type AddressMapper struct {
	phdrs []addressMapperPHDR
}

var pageSizeMinusOne = uint64(os.Getpagesize()) - 1

// FileOffsetToVirtualAddress attempts to convert an on-disk file offset to the
// ELF virtual address where it would be mapped by default.
func (am *AddressMapper) FileOffsetToVirtualAddress(fileOffset uint64) (uint64, bool) {
	for _, p := range am.phdrs {
		//nolint:lll
		// fileOffset may not correspond to any file offset present in the ELF program headers.
		// Indeed, mmap alignment constraints may have forced the ELF loader to start a segment
		// mapping before the actual start of the ELF LOAD segment. Because of this, we must
		// perform the alignment logic on the offsets from the ELF program headers before comparing
		// them to the fileOffset.
		// Both [1] the kernel and [2] glibc (during dynamic linking) use the system page size when
		// performing the alignment. Here we must replicate the same logic, hoping that no ELF
		// loaders would do things differently (one could use a greater multiple of the page size
		// when the ELF allows it, for example when p_align > pageSize).
		// [1]: https://elixir.bootlin.com/linux/v5.10/source/fs/binfmt_elf.c#L367
		// [2]: https://github.com/bminor/glibc/blob/99468ed45f5a58f584bab60364af937eb6f8afda/elf/dl-load.c#L1159
		alignedOffset := p.offset &^ pageSizeMinusOne

		// Check if the offset corresponds to the current segment
		if fileOffset >= alignedOffset && fileOffset < p.offset+p.filesz {
			// Return the page-aligned Vaddr
			return p.vaddr - (p.offset - fileOffset), true
		}
	}
	return 0, false
}

// NewAddressMapper returns an address mapper for given ELF File
func (f *File) GetAddressMapper() AddressMapper {
	phdrs := make([]addressMapperPHDR, 0, 1)
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Flags&elf.PF_X == 0 {
			continue
		}
		phdrs = append(phdrs, addressMapperPHDR{
			offset: p.ProgHeader.Off,
			vaddr:  p.ProgHeader.Vaddr,
			filesz: p.ProgHeader.Filesz,
		})
	}
	return AddressMapper{phdrs: phdrs}
}
