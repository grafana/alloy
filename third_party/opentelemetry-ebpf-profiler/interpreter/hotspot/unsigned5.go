// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

import (
	"fmt"
	"io"
)

// unsigned5Decoder is a decoder for UNSIGNED5 based byte streams.
type unsigned5Decoder struct {
	// r is the byte reader interface to read from
	r io.ByteReader

	// x is the number of exclusion bytes in encoding (JDK20+)
	x uint8
}

// getUint decodes one "standard" J2SE Pack200 UNSIGNED5 number
func (d *unsigned5Decoder) getUint() (uint32, error) {
	const L = uint8(192)
	x := d.x
	r := d.r

	ch, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if ch < x {
		return 0, fmt.Errorf("byte %#x is in excluded range", ch)
	}

	sum := uint32(ch - x)
	for shift := 6; ch >= L && shift < 30; shift += 6 {
		ch, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		if ch < x {
			return 0, fmt.Errorf("byte %#x is in excluded range", ch)
		}
		sum += uint32(ch-x) << shift
	}
	return sum, nil
}

// getSigned decodes one signed number
func (d *unsigned5Decoder) getSigned() (int32, error) {
	val, err := d.getUint()
	if err != nil {
		return 0, err
	}
	return int32(val>>1) ^ -int32(val&1), nil
}

// decodeLineTableEntry incrementally parses one line-table entry consisting of the source
// line number and a byte code index (BCI) from the decoder. The delta encoded line
// table format is specific to HotSpot VM which compresses the unpacked class file line
// tables during class loading.
func (d *unsigned5Decoder) decodeLineTableEntry(bci, line *uint32) error {
	b, err := d.r.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read line table: %v", err)
	}
	switch b {
	case 0x00: // End-of-Stream
		return io.EOF
	case 0xff: // Escape for long deltas
		val, err := d.getSigned()
		if err != nil {
			return fmt.Errorf("failed to read byte code index delta: %v", err)
		}
		*bci += uint32(val)
		val, err = d.getSigned()
		if err != nil {
			return fmt.Errorf("failed to read line number delta: %v", err)
		}
		*line += uint32(val)
	default: // Short encoded delta
		*bci += uint32(b >> 3)
		*line += uint32(b & 7)
	}
	return nil
}

// mapByteCodeIndexToLine decodes a line table to map a given Byte Code Index (BCI)
// to a line number
func (d *unsigned5Decoder) mapByteCodeIndexToLine(bci uint32) uint32 {
	// The line numbers array is a short array of 2-tuples [start_pc, line_number].
	// Not necessarily sorted. Encoded as delta-encoded numbers.
	var curBci, curLine, bestBci, bestLine uint32

	for d.decodeLineTableEntry(&curBci, &curLine) == nil {
		if curBci == bci {
			return curLine
		}
		if curBci >= bestBci && curBci < bci {
			bestBci = curBci
			bestLine = curLine
		}
	}
	return bestLine
}
