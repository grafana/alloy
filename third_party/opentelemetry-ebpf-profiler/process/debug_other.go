//go:build !linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"fmt"
	"runtime"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// NewPtrace is the stub implementation, allowing to compile the process
// package on non linux systems, always failing at runtime with an error if used.
func NewPtrace(_ libpf.PID) (Process, error) {
	return nil, fmt.Errorf("unsupported os %s", runtime.GOOS)
}
