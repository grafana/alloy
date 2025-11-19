// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	nativeTableBlockSize = uint32(16)
	nativeTableBlockMask = nativeTableBlockSize - 1
)

// Ready-to-Run Native Format Reader
// https://github.com/dotnet/runtime/blob/v8.0.0/src/coreclr/vm/nativeformatreader.h
type nativeReader struct {
	io.ReaderAt
}

// UintFixed reads a fixed size integer of 'n' bytes at position offs.
func (nr *nativeReader) UintFixed(offs int64, n int) (value uint32, newOffs int64, err error) {
	var d [4]byte
	_, err = nr.ReadAt(d[0:n], offs)
	return binary.LittleEndian.Uint32(d[:]), offs + int64(n), err
}

// Uint decodes one Native encoded Unsigned integer at position offs.
// https://github.com/dotnet/runtime/blob/v8.0.0/src/coreclr/vm/nativeformatreader.h#L104
func (nr *nativeReader) Uint(offs int64) (value uint32, newOffs int64, err error) {
	var data [4]byte
	if _, err = nr.ReadAt(data[0:1], offs); err != nil {
		return 0, offs, err
	}
	var more uint8
	switch {
	case data[0]&0x01 == 0:
		return uint32(data[0] >> 1), offs + 1, nil
	case data[0]&0x02 == 0:
		more = 1
	case data[0]&0x04 == 0:
		more = 2
	case data[0]&0x08 == 0:
		more = 3
	case data[0]&0x10 == 0:
		return nr.UintFixed(offs+1, 4)
	default:
		return 0, offs, fmt.Errorf("invalid native uint format byte %#02x", data[0])
	}
	_, err = nr.ReadAt(data[1:more+1], offs+1)
	return binary.LittleEndian.Uint32(data[:]) >> (more + 1), offs + int64(more) + 1, err
}

// decodeBlock handles one bit of native sparse array block walking.
func (nr *nativeReader) decodeBlock(offset int64, index, bitMask uint32,
	cb func(uint32, int64) error) error {
	if bitMask == 0 {
		return cb(index, offset)
	}

	val, nextOffset, err := nr.Uint(offset)
	if err != nil {
		return err
	}
	if val&1 != 0 {
		// Left entry valid, recurse with bit not set.
		err = nr.decodeBlock(nextOffset, index, bitMask>>1, cb)
		if err != nil {
			return err
		}
	}
	if val&2 != 0 {
		// Right entry valid, recurse with bit set.
		err = nr.decodeBlock(offset+int64(val>>2), index|bitMask, bitMask>>1, cb)
		if err != nil {
			return err
		}
	}
	if val&3 == 0 && val < 0x40 {
		// Special entry: only the entry encoded in "val" entry is present.
		// And a special tombstone meaning no entry in this block if the "val"
		// is larger than block.
		err = nr.decodeBlock(nextOffset, (index&^nativeTableBlockMask)|(val>>2), 0, cb)
		if err != nil {
			return err
		}
	}
	return nil
}

// WalkTable enumerates all entries in a sparse "Native Table". The R2RFMT does not document the
// format, there is just a TODO placeholder item. The code to lookup an item in a NativeTable
// is at: https://github.com/dotnet/runtime/blob/v8.0.0/src/coreclr/vm/nativeformatreader.h#L370
func (nr *nativeReader) WalkTable(cb func(uint32, int64) error) error {
	header, baseOffset, err := nr.Uint(0)
	if err != nil {
		return err
	}
	entrySize := int(0)
	switch header & 3 {
	case 0:
		entrySize = 1
	case 1:
		entrySize = 2
	default:
		entrySize = 4
	}
	numElems := header >> 2

	indexOffset := baseOffset
	for i := uint32(0); i < numElems; i += nativeTableBlockSize {
		var blockOffset uint32
		blockOffset, indexOffset, err = nr.UintFixed(indexOffset, entrySize)
		if err != nil {
			return err
		}
		err = nr.decodeBlock(baseOffset+int64(blockOffset), i, nativeTableBlockSize>>1, cb)
		if err != nil {
			return err
		}
	}
	return nil
}
