// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package php // import "go.opentelemetry.io/ebpf-profiler/interpreter/php"

import (
	"errors"

	"go.opentelemetry.io/ebpf-profiler/asm/amd"
	e "go.opentelemetry.io/ebpf-profiler/asm/expression"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"golang.org/x/arch/x86/x86asm"
)

// retrieveZendVMKindX86. This function reads the code blob and recovers
// the type of the PHP VM that is used by this process.
func retrieveZendVMKindX86(code []byte) (uint, error) {
	it := amd.NewInterpreterWithCode(code)
	_, err := it.LoopWithBreak(func(op x86asm.Inst) bool {
		return op.Op == x86asm.RET
	})
	if err != nil {
		return 0, err
	}
	res := it.Regs.Get(amd.RAX)
	imm := e.NewImmediateCapture("imm")
	if res.Match(imm) {
		return uint(imm.CapturedValue()), nil
	}
	return 0, errors.New("failed to decode zend_vm_kind")
}

// retrieveExecuteExJumpLabelAddressX86 reads the code blob and returns
// the address of the return address for any JIT code called from execute_ex. Since all JIT
// code is ultimately called from execute_ex, this is the same as returning the return address
// for all JIT code.
func retrieveExecuteExJumpLabelAddressX86(code []byte, addrBase libpf.SymbolValue) (
	libpf.SymbolValue, error) {
	it := amd.NewInterpreterWithCode(code)
	it.CodeAddress = e.Imm(uint64(addrBase))
	_, err := it.LoopWithBreak(func(op x86asm.Inst) bool {
		return op.Op == x86asm.JMP
	})
	if err != nil {
		return 0, err
	}
	res := it.Regs.Get(amd.RIP)
	imm := e.NewImmediateCapture("imm")
	if res.Match(imm) {
		return libpf.SymbolValue(imm.CapturedValue()), nil
	}
	return libpf.SymbolValueInvalid, errors.New("failed to decode execute_ex")
}

// retrieveJITBufferPtrx86 reads the code blob and returns a pointer
// to the JIT buffer used by PHP (called "dasm_buf" in the PHP source).
func retrieveJITBufferPtrx86(code []byte, addrBase libpf.SymbolValue) (
	dasmBuf libpf.SymbolValue, dasmSize libpf.SymbolValue, err error) {
	it := amd.NewInterpreterWithCode(code)
	it.CodeAddress = e.Imm(uint64(addrBase))
	_, err = it.LoopWithBreak(func(op x86asm.Inst) bool {
		return op.Op == x86asm.CALL
	})
	if err != nil {
		return 0, 0, err
	}
	rdi := e.NewImmediateCapture("rdi")
	rsi := e.NewImmediateCapture("rsi")
	if it.Regs.Get(amd.RDI).Match(e.Mem8(rdi)) &&
		it.Regs.Get(amd.RSI).Match(e.Mem8(rsi)) {
		rdiValue := libpf.SymbolValue(rdi.CapturedValue())
		rsiValue := libpf.SymbolValue(rsi.CapturedValue())
		return rdiValue, rsiValue, nil
	}
	return libpf.SymbolValueInvalid, libpf.SymbolValueInvalid,
		errors.New("failed to recover jit buffer")
}
