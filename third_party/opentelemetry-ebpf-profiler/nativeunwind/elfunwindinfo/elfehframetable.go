// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo // import "go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"

import (
	"debug/elf"
	"errors"
	"fmt"
	"sort"
	"unsafe"

	lru "github.com/elastic/go-freelru"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
)

type FDE struct {
	PCBegin uintptr
	PCRange uintptr
}

type EhFrameTable struct {
	fdeCount       uintptr
	tableStartPos  uintptr
	tableEntrySize int
	tableEnc       encoding
	ehFrameHdrSec  *elfRegion
	ehFrameSec     *elfRegion
	efm            elf.Machine

	// cieCache holds the CIEs decoded so far. This is the only piece that is
	// not concurrent safe, and could be made into a sync lru if needed.
	cieCache *lru.LRU[uint64, *cieInfo]
}

// NewEhFrameTable creates a new EhFrameTable from the given pfelf.File.
// The returned EhFrameTable is not concurrent safe.
func NewEhFrameTable(ef *pfelf.File) (*EhFrameTable, error) {
	ehFrameHdrSec, ehFrameSec, err := findEhSections(ef)
	if err != nil {
		return nil, fmt.Errorf("failed to get EH sections: %w", err)
	}
	if ehFrameSec == nil {
		return nil, errors.New(".eh_frame not found")
	}
	if ehFrameHdrSec == nil {
		return nil, errors.New(".eh_frame_hdr not found")
	}
	return newEhFrameTableFromSections(ehFrameHdrSec, ehFrameSec, ef.Machine)
}

// LookupFDE performs a binary search in .eh_frame_hdr for an FDE covering the given addr.
func (e *EhFrameTable) LookupFDE(addr libpf.Address) (FDE, error) {
	idx := sort.Search(e.count(), func(idx int) bool {
		r := e.entryAt(idx)
		ipStart, _ := r.ptr(e.tableEnc) // ignoring error, check bounds later
		return ipStart > uintptr(addr)
	})
	if idx <= 0 {
		return FDE{}, errors.New("FDE not found")
	}
	ipStart, fr, entryErr := e.decodeEntryAt(idx - 1)
	if entryErr != nil {
		return FDE{}, entryErr
	}
	_, fde, _, err := parsesFDEHeader(&fr, e.efm, ipStart, e.cieCache)
	if err != nil {
		return FDE{}, err
	}
	if uintptr(addr) < fde.ipStart || uintptr(addr) >= fde.ipStart+fde.ipLen {
		return FDE{}, errors.New("FDE not found")
	}

	return FDE{
		PCBegin: fde.ipStart,
		PCRange: fde.ipLen,
	}, nil
}

func newEhFrameTableFromSections(ehFrameHdrSec *elfRegion,
	ehFrameSec *elfRegion, efm elf.Machine,
) (hp *EhFrameTable, err error) {
	hdr := (*ehFrameHdr)(unsafe.Pointer(&ehFrameHdrSec.data[0]))

	r := ehFrameHdrSec.reader(unsafe.Sizeof(ehFrameHdr{}), false)
	if _, err = r.ptr(hdr.ehFramePtrEnc); err != nil {
		return nil, err
	}
	fdeCount, err := r.ptr(hdr.fdeCountEnc)
	if err != nil {
		return nil, err
	}
	cieCache, err := lru.New[uint64, *cieInfo](cieCacheSize, hashUint64)
	if err != nil {
		return nil, err
	}
	return &EhFrameTable{
		fdeCount:       fdeCount,
		tableStartPos:  r.pos,
		tableEntrySize: formatLen(hdr.tableEnc) * 2,
		tableEnc:       hdr.tableEnc,
		ehFrameHdrSec:  ehFrameHdrSec,
		ehFrameSec:     ehFrameSec,
		efm:            efm,
		cieCache:       cieCache,
	}, nil
}

// returns FDE count
func (e *EhFrameTable) count() int {
	return int(e.fdeCount)
}

// entryAt returns a reader for the binary search table at given index.
func (e *EhFrameTable) entryAt(idx int) reader {
	return e.ehFrameHdrSec.reader(e.tableStartPos+uintptr(e.tableEntrySize*idx), false)
}

// decodeEntry decodes one entry of the binary search table from the reader.
func (e *EhFrameTable) decodeEntry(r *reader) (ipStart uintptr, fr reader, err error) {
	ipStart, err = r.ptr(e.tableEnc)
	if err != nil {
		return 0, reader{}, err
	}
	var fdeAddr uintptr
	fdeAddr, err = r.ptr(e.tableEnc)
	if err != nil {
		return 0, reader{}, err
	}
	if fdeAddr < e.ehFrameSec.vaddr {
		return 0, reader{}, fmt.Errorf("FDE %#x before section start %#x",
			fdeAddr, e.ehFrameSec.vaddr)
	}
	fr = e.ehFrameSec.reader(fdeAddr-e.ehFrameSec.vaddr, false)
	return ipStart, fr, err
}

// decodeEntryAt decodes the entry from given index.
func (e *EhFrameTable) decodeEntryAt(idx int) (ipStart uintptr, fr reader, err error) {
	r := e.entryAt(idx)
	return e.decodeEntry(&r)
}

// formatLen returns the length of a field encoded with enc encoding.
func formatLen(enc encoding) int {
	switch enc & encFormatMask {
	case encFormatData2:
		return 2
	case encFormatData4:
		return 4
	case encFormatData8, encFormatNative:
		return 8
	default:
		return 0
	}
}
