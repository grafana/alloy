//go:build linux && amd64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

func (sp *ptraceProcess) getThreadInfo(tid int) (ThreadInfo, error) {
	prStatus := make([]byte, 28*8)
	if err := ptraceGetRegset(tid, int(elf.NT_PRSTATUS), prStatus); err != nil {
		return ThreadInfo{}, fmt.Errorf("failed to get LWP %d thread info: %v", tid, err)
	}
	return ThreadInfo{
		LWP:    uint32(tid),
		GPRegs: prStatus,
		TPBase: binary.LittleEndian.Uint64(prStatus[21*8:]),
	}, nil
}
