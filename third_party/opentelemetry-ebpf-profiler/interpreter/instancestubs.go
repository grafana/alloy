// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interpreter // import "go.opentelemetry.io/ebpf-profiler/interpreter"

import (
	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/tpbase"
)

// InstanceStubs provides empty implementations of Instance hooks that are
// not mandatory for a Instance implementation.
type InstanceStubs struct {
}

func (is *InstanceStubs) SynchronizeMappings(EbpfHandler, reporter.ExecutableReporter,
	process.Process, []process.Mapping) error {
	return nil
}

func (is *InstanceStubs) UpdateTSDInfo(EbpfHandler, libpf.PID, tpbase.TSDInfo) error {
	return nil
}

func (is *InstanceStubs) Symbolize(*host.Frame, *libpf.Frames) error {
	return ErrMismatchInterpreterType
}

func (is *InstanceStubs) GetAndResetMetrics() ([]metrics.Metric, error) {
	return []metrics.Metric{}, nil
}

func (is *InstanceStubs) ReleaseResources() error {
	return nil
}
