// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// This package contains a series of helper functions that are useful for ARM disassembly.
package armhelpers // import "go.opentelemetry.io/ebpf-profiler/armhelpers"

import (
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/ebpf-profiler/stringutil"
	aa "golang.org/x/arch/arm64/arm64asm"
)

// Xreg2num converts arm64asm Reg or RegSP X0...X30 and W0...W30 register enum into a register
// number. X0/W0 return 0, X1/W1 return 1, etc.
func Xreg2num(arg any) (int, bool) {
	var ndx aa.Reg
	switch reg := arg.(type) {
	case aa.Reg:
		ndx = reg
	case aa.RegSP:
		ndx = aa.Reg(reg)
	case aa.RegExtshiftAmount:
		// Similar to other instructions, fields of RegExtshiftAmount are not exported.
		// https://github.com/golang/go/issues/51517
		n, ok := DecodeRegister(arg.(aa.RegExtshiftAmount).String())
		if !ok {
			return 0, false
		}
		ndx = n
	default:
		return 0, false
	}

	switch {
	case ndx >= aa.X0 && ndx <= aa.X30:
		return int(ndx - aa.X0), true
	case ndx >= aa.W0 && ndx <= aa.W30:
		return int(ndx - aa.W0), true
	}

	return 0, false
}

// DecodeRegister converts the result of calling Reg.String()
// into the initial register's value.
func DecodeRegister(reg string) (aa.Reg, bool) {
	const maxRegister = uint64(aa.V31)

	// This function is essentially just the inverse
	// of https://cs.opensource.google/go/x/arch/+/fc48f9fe:arm64/arm64asm/inst.go;l=335
	length := len(reg)
	if length == 0 {
		return 0, false
	}

	// WZR and XZR don't have a value.
	if reg == "WZR" || reg == "XZR" {
		return 0, false
	}

	// The special case is having a string containing Reg(%d).
	if length > 3 && reg[0:2] == "Reg" {
		val, err := strconv.ParseUint(reg[3:length-1], 10, 64)
		if err != nil {
			return 0, false
		}
		if val > maxRegister {
			return 0, false
		}
		return aa.Reg(val), true
	}

	// Otherwise, we want to strip out the
	// leading character only if the
	// character is one of a few.
	var regOffset uint64
	switch reg[0] {
	case 'W':
		regOffset = uint64(aa.W0)
	case 'X':
		regOffset = uint64(aa.X0)
	case 'B':
		regOffset = uint64(aa.B0)
	case 'H':
		regOffset = uint64(aa.H0)
	case 'S':
		regOffset = uint64(aa.S0)
	case 'D':
		regOffset = uint64(aa.D0)
	case 'Q':
		regOffset = uint64(aa.Q0)
	case 'V':
		regOffset = uint64(aa.V0)
	default:
		return 0, false
	}

	val, err := strconv.ParseUint(reg[1:], 10, 64)
	if err != nil {
		return 0, false
	}

	res := val + regOffset
	if res > maxRegister {
		return 0, false
	}
	return aa.Reg(res), true
}

// DecodeImmediate converts an arm64asm Arg of immediate type to it's value.
func DecodeImmediate(arg aa.Arg) (int64, bool) {
	switch val := arg.(type) {
	case aa.Imm:
		return int64(val.Imm), true
	case aa.PCRel:
		return int64(val), true
	case aa.MemImmediate:
		// The MemImmediate layout changes quite
		// a bit depending on its mode.
		var fields [2]string
		// All of the strings we are formatted in the following way:
		// 1) They all (except 1) contain a comma
		// 2) They all (except 1) have the offset as the second parameter.
		// The exception is aa.AddrOffset when the offset is 0.
		n := stringutil.SplitN(val.String(), ",", fields[:])
		if n == 0 || n > 2 {
			// This is a failure case
			return 0, false
		}

		if n == 1 {
			// This should happen only if we have an AddrOffset where there's
			// a 0 offset. See
			// https://cs.opensource.google/go/x/arch/+/fc48f9fe:arm64/arm64asm/inst.go;l=515
			return 0, true
		}

		// In all other cases we want to split the string around the comma and
		// extract the second number. Note that the string will start with a #
		// in all but one case (namely, AddrPostReg).
		pos := strings.Index(fields[1], "#")
		if pos == -1 {
			// We have a string that looks like this:
			// [%s], %s
			// Note that the second %s here is the print
			// format from a register. Annoyingly this isn't a
			// register type, so we have to unwind it manually
			reg, ok := DecodeRegister(fields[1])
			if !ok {
				return 0, false
			}
			// The Go disassembler always adds X0 here.
			// See https://cs.opensource.google/go/x/arch/+/fc48f9fe:arm64/arm64asm/inst.go;l=526
			return int64(reg - aa.X0), true
		}

		// Otherwise all of the strings end with a ], so we just parse
		// the string before that.
		endIndex := strings.Index(fields[1], "]")
		// The strings are base 10 encoded
		out, err := strconv.ParseInt(fields[1][pos+1:endIndex], 10, 64)
		if err != nil {
			return 0, false
		}
		return out, true

	case aa.ImmShift:
		// Sadly, ImmShift{} does not have public fields.
		// https://github.com/golang/go/issues/51517
		var imm int64
		n, err := fmt.Sscanf(val.String(), "#%v", &imm)
		if err != nil || n != 1 {
			return 0, false
		}
		return imm, true
	}

	return 0, false
}
