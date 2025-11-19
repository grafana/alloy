// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo // import "go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"

import (
	"debug/elf"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
)

const (
	// Some DSOs have few limited .eh_frame FDEs (e.g. PLT), and additional
	// FDEs are in .debug_frame or external debug file. This controls how many
	// intervals are needed to not follow .gnu_debuglink.
	numIntervalsToOmitDebugLink = 20
)

// extractionFilter is used to filter in .eh_frame data when a better source
// is available (.gopclntab).
type extractionFilter struct {
	// start and end contains the virtual address block of code which
	// should be excluded from .eh_frame extraction.
	start, end uintptr

	// entryStart and entryEnd contain the virtual address for the entry
	// stub code with synthesized stack deltas.
	entryStart, entryEnd uintptr

	// entryPending is true if the entry stub stack delta has not been added.
	entryPending bool

	// ehFrames is true if .eh_frame stack deltas are found
	ehFrames bool

	// golangFrames is true if .gopclntab stack deltas are found
	golangFrames bool

	// unsortedFrames is set if stack deltas from unsorted source are found
	unsortedFrames bool
}

var _ ehframeHooks = &extractionFilter{}

// addEntryDeltas generates the entry stub stack deltas.
func (f *extractionFilter) addEntryDeltas(deltas *sdtypes.StackDeltaArray) {
	deltas.AddEx(sdtypes.StackDelta{
		Address: uint64(f.entryStart),
		Hints:   sdtypes.UnwindHintKeep,
		Info:    sdtypes.UnwindInfoStop,
	}, !f.unsortedFrames)
	deltas.Add(sdtypes.StackDelta{
		Address: uint64(f.entryEnd),
		Info:    sdtypes.UnwindInfoInvalid,
	})
	f.ehFrames = true
	f.entryPending = false
}

func (f *extractionFilter) fdeUnsorted() {
	f.unsortedFrames = true
}

// fdeHook filters out .eh_frame data that is superseded by .gopclntab data
func (f *extractionFilter) fdeHook(_ *cieInfo, fde *fdeInfo, deltas *sdtypes.StackDeltaArray) bool {
	// Drop FDEs inside the gopclntab area
	if f.start <= fde.ipStart && fde.ipStart+fde.ipLen <= f.end {
		return false
	}
	// Seems .debug_frame sometimes has broken FDEs for zero address
	if f.unsortedFrames && fde.ipStart == 0 {
		return false
	}
	// Insert entry stub deltas to their sorted position.
	if f.entryPending && fde.ipStart >= f.entryStart {
		f.addEntryDeltas(deltas)
	}
	// Drop FDEs overlapping with the detected entry stub.
	if fde.ipStart+fde.ipLen > f.entryStart && f.entryEnd >= fde.ipStart {
		return false
	}
	// This is here to set the flag only when we have collected at least
	// one stack delta from the relevant source.
	f.ehFrames = true
	return true
}

// deltaHook is a stub to satisfy ehframeHooks interface
func (f *extractionFilter) deltaHook(uintptr, *vmRegs, sdtypes.StackDelta) {
}

// golangHook reports the .gopclntab area
func (f *extractionFilter) golangHook(start, end uintptr) {
	f.start = start
	f.end = end
	f.golangFrames = true
}

// elfExtractor is the main context for parsing stack deltas from an ELF
type elfExtractor struct {
	ref  *pfelf.Reference
	file *pfelf.File

	hooks ehframeHooks

	deltas *sdtypes.StackDeltaArray

	// allowGenericRegs enables generation of unwinding using specific general purpose
	// registers as CFA base. This is possible for code that does not call into other
	// functions that would trash these registers (we cannot recover these registers
	// during unwind). This is currently enabled for openssl libcrypto only.
	allowGenericRegs bool
}

func (ee *elfExtractor) extractDebugDeltas() (err error) {
	// Attempt finding the associated debug information file with .debug_frame,
	// but ignore errors if it's not available; many production systems
	// do not intentionally have debug packages installed.
	debugELF, _ := ee.file.OpenDebugLink(ee.ref.FileName(), ee.ref)
	if debugELF != nil {
		err = ee.parseDebugFrame(debugELF)
		_ = debugELF.Close()
	}
	return err
}

func isLibCrypto(elfFile *pfelf.File) bool {
	if name, err := elfFile.DynString(elf.DT_SONAME); err == nil && len(name) == 1 {
		// Allow generic register CFA for openssl libcrypto
		return strings.HasPrefix(name[0], "libcrypto.so.")
	}
	return false
}

// Extract takes a filename for a modern ELF file that is accessible
// and provides the stack delta intervals in the interval parameter
func Extract(filename string, interval *sdtypes.IntervalData) error {
	elfRef := pfelf.NewReference(filename, pfelf.SystemOpener)
	defer elfRef.Close()
	return ExtractELF(elfRef, interval)
}

// detectEntryCode matches machine code for known entry stubs, and detects its length.
func detectEntryCode(machine elf.Machine, code []byte) int {
	switch machine {
	case elf.EM_X86_64:
		return detectEntryX86(code)
	case elf.EM_AARCH64:
		return detectEntryARM(code)
	default:
		return 0
	}
}

// detectEntry loads the entry stub from the ELF DSO entry and matches it.
func detectEntry(ef *pfelf.File) int {
	if ef.Entry == 0 {
		return 0
	}

	// Typically 52-80 bytes, allow for a bit of variance
	code, err := ef.VirtualMemory(int64(ef.Entry), 128, 128)
	if err != nil {
		return 0
	}
	return detectEntryCode(ef.Machine, code)
}

// ExtractELF takes a pfelf.Reference and provides the stack delta
// intervals for it in the interval parameter.
func ExtractELF(elfRef *pfelf.Reference, interval *sdtypes.IntervalData) error {
	elfFile, err := elfRef.GetELF()
	if err != nil {
		return err
	}
	return extractFile(elfFile, elfRef, interval)
}

// extractFile extracts the elfFile stack deltas and uses the optional elfRef to resolve
// debug link references if needed.
func extractFile(elfFile *pfelf.File, elfRef *pfelf.Reference,
	interval *sdtypes.IntervalData) (err error) {
	// Parse the stack deltas from the ELF
	filter := extractionFilter{}
	deltas := sdtypes.StackDeltaArray{}
	ee := elfExtractor{
		ref:              elfRef,
		file:             elfFile,
		deltas:           &deltas,
		hooks:            &filter,
		allowGenericRegs: isLibCrypto(elfFile),
	}

	if entryLength := detectEntry(elfFile); entryLength != 0 {
		filter.entryStart = uintptr(elfFile.Entry)
		filter.entryEnd = filter.entryStart + uintptr(entryLength)
		filter.entryPending = true
	}

	if err = ee.parseGoPclntab(); err != nil {
		return fmt.Errorf("failure to parse golang stack deltas: %v", err)
	}
	if err = ee.parseEHFrame(); err != nil {
		return fmt.Errorf("failure to parse eh_frame stack deltas: %v", err)
	}
	if err = ee.parseDebugFrame(elfFile); err != nil {
		return fmt.Errorf("failure to parse debug_frame stack deltas: %v", err)
	}
	if ee.ref != nil && len(deltas) < numIntervalsToOmitDebugLink {
		// There is only few stack deltas. See if we find the .gnu_debuglink
		// debug information for additional .debug_frame stack deltas.
		if err = ee.extractDebugDeltas(); err != nil {
			return fmt.Errorf("failure to parse debug stack deltas: %v", err)
		}
	}
	if filter.entryPending {
		filter.addEntryDeltas(ee.deltas)
	}

	// If multiple sources were merged, sort them.
	if filter.unsortedFrames || (filter.ehFrames && filter.golangFrames) {
		sort.Slice(deltas, func(i, j int) bool {
			if deltas[i].Address != deltas[j].Address {
				return deltas[i].Address < deltas[j].Address
			}
			// Make sure that the potential duplicate "invalid" delta is sorted
			// after the real delta so the proper delta is removed in next stage.
			if deltas[i].Info.Opcode != deltas[j].Info.Opcode {
				return deltas[i].Info.Opcode < deltas[j].Info.Opcode
			}
			return deltas[i].Info.Param < deltas[j].Info.Param
		})

		maxDelta := 0
		for i := 0; i < len(deltas); i++ {
			delta := &deltas[i]
			if maxDelta > 0 {
				// This duplicates the logic from StackDeltaArray.Add()
				// to remove duplicate and redundant stack deltas.
				prev := &deltas[maxDelta-1]
				if prev.Hints&sdtypes.UnwindHintGap != 0 &&
					prev.Address+sdtypes.MinimumGap >= delta.Address {
					// The previous opcode is end-of-function marker, and
					// the gap is not large. Reduce deltas by overwriting it.
					if maxDelta <= 1 || deltas[maxDelta-2].Info != delta.Info {
						*prev = *delta
						continue
					}
					// The delta before end-of-function marker is same as
					// what is being inserted now. Overwrite that.
					prev = &deltas[maxDelta-2]
					maxDelta--
				}
				if prev.Info == delta.Info {
					prev.Hints |= delta.Hints & sdtypes.UnwindHintKeep
					continue
				}
				if prev.Address == delta.Address {
					*prev = *delta
					continue
				}
			}
			deltas[maxDelta] = *delta
			maxDelta++
		}
		deltas = deltas[:maxDelta]
	}

	*interval = sdtypes.IntervalData{
		Deltas: deltas,
	}
	return nil
}
