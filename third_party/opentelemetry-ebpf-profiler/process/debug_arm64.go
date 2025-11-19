//go:build linux && arm64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

func (sp *ptraceProcess) GetMachineData() MachineData {
	pacMask := make([]byte, 16)
	_ = ptraceGetRegset(int(sp.pid), int(NT_ARM_PAC_MASK), pacMask)

	return MachineData{
		Machine:     elf.EM_AARCH64,
		DataPACMask: binary.LittleEndian.Uint64(pacMask[0:8]),
		CodePACMask: binary.LittleEndian.Uint64(pacMask[8:16]),
	}
}

func (sp *ptraceProcess) getThreadInfo(tid int) (ThreadInfo, error) {
	prStatus := make([]byte, 35*8)
	if err := ptraceGetRegset(tid, int(elf.NT_PRSTATUS), prStatus); err != nil {
		return ThreadInfo{}, fmt.Errorf("failed to get LWP %d thread info: %v", tid, err)
	}
	// Treat TLS base reading error as non-fatal
	armTLS := make([]byte, 8)
	_ = ptraceGetRegset(tid, int(NT_ARM_TLS), armTLS)

	return ThreadInfo{
		LWP:    uint32(tid),
		GPRegs: prStatus,
		TPBase: binary.LittleEndian.Uint64(armTLS),
	}, nil
}
