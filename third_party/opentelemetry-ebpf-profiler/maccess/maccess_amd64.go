//go:build amd64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maccess // import "go.opentelemetry.io/ebpf-profiler/maccess"

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// CopyFromUserNoFaultIsPatched tries to find a relative jump instruction in codeblob
// and returns true if this jump based on faultyFuncAddr points to newCheckFuncAddr.
func CopyFromUserNoFaultIsPatched(codeblob []byte,
	faultyFuncAddr uint64, newCheckFuncAddr uint64) (bool, error) {
	if len(codeblob) == 0 {
		return false, errors.New("empty code blob")
	}
	if newCheckFuncAddr == 0 {
		return false, errors.New("nmi_uaccess_okay function not found")
	}

	for i := 0; i < len(codeblob); {
		idx, offset := getRelativeOffset(codeblob[i:])
		if idx < 0 {
			break
		}

		// Sanity check:
		// Check whether this is a call to `nmi_uaccess_okay`.
		// The offset in a relative jump instruction is relative to the start of the next
		// instruction (i+idx+5).
		if faultyFuncAddr+uint64(i)+uint64(idx)+uint64(offset)+5 == newCheckFuncAddr {
			return true, nil
		}

		// Start looking for the next relative jump instruction in codeblob after the
		// current finding.
		i += idx + 1
	}
	return false, nil
}

// getRelativeOffset looks for the E8 call instruction in codeblob and returns the index at which
// this instruction was found first and the relative offset value from this instruction.
func getRelativeOffset(codeblob []byte) (idx int, offset int32) {
	idx = bytes.Index(codeblob, []byte{0xe8})
	if idx == -1 || idx+5 > len(codeblob) {
		return -1, 0
	}
	tmp := binary.LittleEndian.Uint32(codeblob[idx+1:])
	return idx, int32(tmp)
}
