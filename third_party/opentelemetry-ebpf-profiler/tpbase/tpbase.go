// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// tpbase implements disassembly analysis functions to extract needed data for
// the Thread Pointer Base value handling. Code to analyze several Linux Kernel
// architecture specific functions exist to extract offset of the TPBase value
// relative to the 'struct task_struct'. This is needed to support Thread Local
// Storage access in eBPF.

package tpbase // import "go.opentelemetry.io/ebpf-profiler/tpbase"

import (
	"fmt"
	"runtime"
)

func GetAnalyzers() ([]Analyzer, error) {
	switch runtime.GOARCH {
	case "amd64":
		return getAnalyzersX86(), nil
	case "arm64":
		return getAnalyzersARM(), nil
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

type Analyzer struct {
	// FunctionName is the kernel function which can be analyzed
	FunctionName string

	// Analyze can inspect the kernel function mentioned above for the Thread Pointer Base
	Analyze func([]byte) (uint32, error)
}
