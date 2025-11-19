// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"go.opentelemetry.io/ebpf-profiler/armhelpers"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/support"
	aa "golang.org/x/arch/arm64/arm64asm"

	log "github.com/sirupsen/logrus"
)

// nextAligned aligns a pointer up, to the next multiple of align.
func nextAligned(ptr libpf.Address, align uint64) libpf.Address {
	return (ptr + libpf.Address(align)) & ^(libpf.Address(align) - 1)
}

// StubRoutine marks a logical function within the StubRoutines blob.
type StubRoutine struct {
	name       string
	start, end libpf.Address
}

// findStubBounds heuristically determines the bounds of individual functions
// within the larger StubRoutines blobs. We receive pointers for most of the
// stubs from VM structs / JVMCI VM structs, but not the lengths.
//
// This function first collects all routines and sorts them by start address.
// The start of the next stub is then taken as the maximum length of the current
// stub. This works great for most cases, but some functions are missing in VM
// structs (and would thus be assigned to the previous stub), and the last
// function doesn't have anything following it to serve as a boundary.
//
// To handle these edge-cases, we additionally do a sweep for NOP instructions
// that are used as padding between subroutines. One might be inclined to rely
// on this NOP heuristic only, but it's not sufficient alone either: the
// previous stub function might have the perfect length for the next one to
// not need alignment. Also in some cases the JVM devs omitted/forgot to insert
// the padding. The two heuristics combined, however, yield reliable results.
func findStubBounds(vmd *hotspotVMData, bias libpf.Address,
	rm remotememory.RemoteMemory) []StubRoutine {
	const CodeAlign = 64
	const MaxStubLen = 8 * 1024

	stubs := make([]StubRoutine, 0, 64)
	for field, addr := range vmd.vmStructs.StubRoutines.CatchAll {
		if strings.Contains(field, "_table_") {
			continue
		}

		// Not all stubs are generated for all architectures.
		entry := rm.Ptr(addr + bias)
		if entry == 0 {
			continue
		}

		stubs = append(stubs, StubRoutine{
			name:  strings.TrimPrefix(field, "_"),
			start: entry,
			end:   0, // filled in later
		})
	}

	sort.Slice(stubs, func(i, j int) bool {
		if stubs[i].start != stubs[j].start {
			return stubs[i].start < stubs[j].start
		}

		// Secondary ordering by name to ensure that we produce deterministic
		// results even in the presence of stub aliases (same start address).
		return stubs[i].name < stubs[j].name
	})

	filtered := make([]StubRoutine, 0, len(stubs))
	for i := 0; i < len(stubs); i++ {
		cur := &stubs[i]

		// Some stubs reuse the code from another stub. Skip elements until
		// we detected the next stub that doesn't occupy the same address.
		for i < len(stubs) {
			if i != len(stubs)-1 {
				// Beginning of next element marks the maximum length of the
				// previous one.
				next := &stubs[i+1]
				cur.end = next.start
			} else {
				// Last element: assume max length and let the disassembler
				// heuristic below deal with that case.
				cur.end = cur.start + MaxStubLen - 1
			}

			if cur.start == cur.end {
				i++
			} else {
				break
			}
		}

		// Sweep for stub function boundary.
		heuristicEnd := libpf.Address(0)
	NopHeuristic:
		for p := nextAligned(cur.start, CodeAlign); p < cur.start+MaxStubLen; p += CodeAlign {
			const NopARM4 = 0xD503201F
			const NopAMD64 = 0x90

			block := make([]byte, CodeAlign)
			if err := rm.Read(p-CodeAlign, block); err != nil {
				continue
			}

			// Last function in each stub is followed by zeros.
			if libpf.SliceAllEqual(block, 0) {
				heuristicEnd = p
				break NopHeuristic
			}

			// Other functions are separated by NOPs.
			switch runtime.GOARCH {
			case "arm64": //nolint:goconst
				if binary.LittleEndian.Uint32(block[len(block)-4:]) == NopARM4 {
					heuristicEnd = p
					break NopHeuristic
				}
			case "amd64":
				if block[len(block)-1] == NopAMD64 {
					heuristicEnd = p
					break NopHeuristic
				}
			default:
				panic("unexpected architecture")
			}
		}

		// Pick the minimum of both heuristics as length.
		if heuristicEnd != 0 {
			cur.end = min(cur.end, heuristicEnd)
		}

		if cur.end-cur.start > MaxStubLen {
			log.Debugf("Unable to determine length for JVM stub %s", cur.name)
			continue
		}

		filtered = append(filtered, *cur)
	}

	return filtered
}

// analyzeStubArm64 disassembles the first 16 instructions of an ARM64 stub in
// an attempt to detect whether it has a frame or needs an SP offset.
//
// Examples of cases currently handled by this function:
//
// Stack frame setup (checkcast_arraycopy_uninit):
// >>> STP  X29, X30, [SP,#-0x10]!
// >>> MOV  X29, SP
//
// Stack alloc without frame via mutating STP variant (sha256_implCompress):
// >>> STP  D8, D9, [SP,#-0x20]!
//
// Stack alloc with SUB after a few instructions (ghash_processBlocks_wide):
// >>> CMP   X3, #8
// >>> B.LT  loc_4600
// >>> SUB   SP, SP, #0x40
func analyzeStubArm64(rm remotememory.RemoteMemory, addr libpf.Address) (
	hasFrame bool, spOffs int64, err error) {
	code := make([]byte, 64)
	if err := rm.Read(addr, code); err != nil {
		return false, 0, err
	}

Outer:
	for offs := 0; offs < len(code); offs += 4 {
		insn, err := aa.Decode(code[offs : offs+4])
		if err != nil {
			return false, 0, fmt.Errorf("failed to decode instruction: %v", err)
		}

		const SP = aa.RegSP(aa.SP)

		switch insn.Op {
		case aa.STP:
			if insn.Args[0] == aa.X29 && insn.Args[1] == aa.X30 {
				// Assume this is a frame pointer setup.
				return true, 0, nil
			}

			if arg, ok := insn.Args[2].(aa.MemImmediate); ok {
				if arg.Base != SP {
					continue
				}
				if arg.Mode != aa.AddrPostIndex && arg.Mode != aa.AddrPreIndex {
					continue
				}
				imm, ok := armhelpers.DecodeImmediate(arg)
				if !ok {
					continue
				}

				spOffs += imm
			}
		case aa.SUB:
			for _, arg := range insn.Args[:2] {
				if arg, ok := arg.(aa.RegSP); !ok || arg != SP {
					continue Outer
				}
			}
			imm, ok := armhelpers.DecodeImmediate(insn.Args[2])
			if !ok {
				continue
			}

			spOffs -= imm
		}
	}

	return false, spOffs, nil
}

// jitAreaForStubArm64 synthesizes a jitArea for an ARM64 stub routine.
//
// We currently don't make any attempts to generate extra areas for the pro-
// and epilogues of the functions and (incorrectly) assume the SP deltas for
// the duration of the whole function. We expect it to be sufficiently rare
// that sampling catches the pro/epilogues that it isn't really worth special
// casing this any further.
func jitAreaForStubArm64(stub *StubRoutine, heap *jitArea,
	rm remotememory.RemoteMemory) (jitArea, error) {
	var hasFrame bool
	var spOffs int64
	if stub.name == "call_stub_return_address" {
		// Special-case: this is not an actual individual stub function,
		// but rather a pointer into the middle of the call stub.
		hasFrame = true
	} else {
		var err error
		hasFrame, spOffs, err = analyzeStubArm64(rm, stub.start)
		if err != nil {
			return jitArea{}, fmt.Errorf("failed to analyze stub: %v", err)
		}
	}

	tsid := heap.tsid | 1<<support.HSTSIDIsStubBit
	if hasFrame {
		tsid |= 1 << support.HSTSIDHasFrameBit
	} else {
		sd := uint64(-spOffs) / support.HSTSIDStackDeltaScale
		tsid |= sd << support.HSTSIDStackDeltaBit
	}

	return jitArea{
		start:     stub.start,
		end:       stub.end,
		codeStart: heap.codeStart,
		tsid:      tsid,
	}, nil
}
