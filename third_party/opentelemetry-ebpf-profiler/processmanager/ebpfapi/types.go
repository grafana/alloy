// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ebpfapi // import "go.opentelemetry.io/ebpf-profiler/processmanager/ebpfapi"

// StackDeltaEBPF represents stack deltas preprocessed by the ProcessManager which are
// then loaded to the eBPF map. This is Go equivalent of 'struct StackDelta' in eBPF types.h.
// See the eBPF header file for details.
type StackDeltaEBPF struct {
	AddressLow uint16
	UnwindInfo uint16
}
