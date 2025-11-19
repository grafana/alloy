// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// A command-line tool to parse stack deltas from given ELF files. This tool
// can generate statistics on number of stack deltas and different stack delta
// values are seen, or a full listing of stack deltas from a file given with
// -target option.
package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/support"
)

var (
	target = flag.String("target", "", "The target executable to operate on.")

	mergeDistance = flag.Uint("mergeDistance", 2,
		"Maximum distance of stack deltas that can be merged")
)

type stats struct {
	seenDeltas libpf.Set[sdtypes.UnwindInfo]

	numDeltas, numMerged uint
}

func getOpcode(opcode uint8, param int32) string {
	str := ""
	switch opcode &^ support.UnwindOpcodeFlagDeref {
	case support.UnwindOpcodeCommand:
		switch param {
		case support.UnwindCommandInvalid:
			return "invalid"
		case support.UnwindCommandStop:
			return "stop"
		case support.UnwindCommandPLT:
			return "plt"
		case support.UnwindCommandSignal:
			return "signal"
		case support.UnwindCommandFramePointer:
			return "framepointer"
		default:
			return "?"
		}
	case support.UnwindOpcodeBaseCFA:
		str = "cfa"
	case support.UnwindOpcodeBaseFP:
		str = "fp"
	case support.UnwindOpcodeBaseSP:
		str = "sp"
	default:
		return "?"
	}
	if opcode&support.UnwindOpcodeFlagDeref != 0 {
		preDeref, postDeref := sdtypes.UnpackDerefParam(param)
		if postDeref != 0 {
			str = fmt.Sprintf("*(%s%+x)%+x", str, preDeref, postDeref)
		} else {
			str = fmt.Sprintf("*(%s%+x)", str, preDeref)
		}
	} else {
		str = fmt.Sprintf("%s%+x", str, param)
	}
	return str
}

func canMerge(delta, nextDelta sdtypes.StackDelta) bool {
	if nextDelta.Address-delta.Address > uint64(*mergeDistance) {
		return false
	}
	if nextDelta.Info.Opcode != delta.Info.Opcode ||
		nextDelta.Info.FPOpcode != delta.Info.FPOpcode ||
		nextDelta.Info.FPParam != delta.Info.FPParam {
		return false
	}
	deltaDiff := nextDelta.Info.Param - delta.Info.Param
	return deltaDiff >= -8 && deltaDiff <= 8
}

func analyzeFile(filename string, s *stats, dump bool) error {
	var data sdtypes.IntervalData

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %v: %v",
			filename, err)
	}

	if err := elfunwindinfo.Extract(absPath, &data); err != nil {
		return fmt.Errorf("failed to extract stack deltas: %v", err)
	}

	if dump {
		fmt.Printf("%-16v %-16v%-16v %v\n", "# addr", "cfa", "fp", "flags")
	}

	var merged bool
	var numMerged uint
	for index, delta := range data.Deltas {
		if dump {
			cfa := getOpcode(delta.Info.Opcode, delta.Info.Param)
			fp := getOpcode(delta.Info.FPOpcode, delta.Info.FPParam)
			comment := ""
			if delta.Hints&sdtypes.UnwindHintKeep != 0 {
				comment += " keep"
			}
			if delta.Hints&sdtypes.UnwindHintGap != 0 {
				comment += " gap"
			}
			if merged {
				comment += " merged"
			}
			fmt.Printf("%016x %-16s%-16s%s\n", delta.Address, cfa, fp, comment)
		}
		if merged {
			merged = false
			continue
		}
		info := delta.Info
		if index+1 < len(data.Deltas) && canMerge(delta, data.Deltas[index+1]) {
			nextDelta := data.Deltas[index+1]
			merged = true
			numMerged++
			info.MergeOpcode = uint8(nextDelta.Address - delta.Address)
			if nextDelta.Info.Param-delta.Info.Param < 0 {
				info.MergeOpcode |= support.MergeOpcodeNegative
			}
		}
		s.seenDeltas[info] = libpf.Void{}
	}
	numDeltas := uint(len(data.Deltas))
	s.numDeltas += numDeltas
	s.numMerged += numMerged

	fmt.Printf("# %v: %v deltas, %d (%.1f%%) merged\n",
		filename,
		numDeltas,
		numDeltas-numMerged,
		100*float32(numDeltas-numMerged)/float32(numDeltas))

	return nil
}

func main() {
	s := stats{
		seenDeltas: make(libpf.Set[sdtypes.UnwindInfo]),
	}

	flag.Parse()

	if *target != "" {
		if err := analyzeFile(*target, &s, true); err != nil {
			fmt.Printf("# %s: %s\n", *target, err)
		}
	}
	for _, f := range flag.Args() {
		if err := analyzeFile(f, &s, false); err != nil {
			fmt.Printf("# %s: %s\n", f, err)
		}
	}
	fmt.Printf("# %v deltas, %v (%.1f%%) merged, %v unique\n",
		s.numDeltas,
		s.numDeltas-s.numMerged,
		100*float32(s.numDeltas-s.numMerged)/float32(s.numDeltas),
		len(s.seenDeltas))
}
