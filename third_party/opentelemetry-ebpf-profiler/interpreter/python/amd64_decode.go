// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package python // import "go.opentelemetry.io/ebpf-profiler/interpreter/python"

import (
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/asm/amd"
	e "go.opentelemetry.io/ebpf-profiler/asm/expression"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"golang.org/x/arch/x86/x86asm"
)

func decodeStubArgumentAMD64(
	code []byte,
	codeAddress, memBase uint64,
) (
	libpf.SymbolValue, error,
) {
	it := amd.NewInterpreterWithCode(code)
	it.CodeAddress = e.Imm(codeAddress)
	_, err := it.LoopWithBreak(func(op x86asm.Inst) bool {
		return op.Op == x86asm.JMP || op.Op == x86asm.CALL
	})
	if err != nil {
		return 0, err
	}
	answer, err := evaluateStubAnswerAMD64(it.Regs.Get(amd.RDI), memBase)
	if err != nil {
		return 0, err
	}
	return libpf.SymbolValue(answer), err
}

func evaluateStubAnswerAMD64(res e.Expression, memBase uint64) (uint64, error) {
	answer := e.NewImmediateCapture("answer")
	if res.Match(e.ZeroExtend32(e.Mem8(answer))) {
		return answer.CapturedValue(), nil
	}
	if res.Match(
		e.Add(
			e.Mem8(e.NewImmediateCapture("mem")),
			answer,
		),
	) {
		return memBase + answer.CapturedValue(), nil
	}
	if res.Match(
		e.ZeroExtend32(
			e.Mem8(
				e.Add(
					e.Mem8(e.NewImmediateCapture("mem")),
					answer,
				),
			),
		),
	) {
		return memBase + answer.CapturedValue(), nil
	}
	if res.Match(answer) {
		return answer.CapturedValue(), nil
	}
	return 0, fmt.Errorf("not found %s", res.DebugString())
}
