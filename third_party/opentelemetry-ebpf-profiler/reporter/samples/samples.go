// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package samples // import "go.opentelemetry.io/ebpf-profiler/reporter/samples"

import (
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

type TraceEventMeta struct {
	Timestamp      libpf.UnixTime64
	Comm           string
	ProcessName    string
	ExecutablePath string
	APMServiceName string
	ContainerID    string
	PID, TID       libpf.PID
	CPU            int
	Origin         libpf.Origin
	OffTime        int64
	EnvVars        map[string]string
}

// TraceEvents holds known information about a trace.
type TraceEvents struct {
	Frames     libpf.Frames
	Timestamps []uint64 // in nanoseconds
	OffTimes   []int64  // in nanoseconds
	EnvVars    map[string]string
	Labels     map[string]string
}

// TraceAndMetaKey is the deduplication key for samples. This **must always**
// contain all trace fields that aren't already part of the trace hash to ensure
// that we don't accidentally merge traces with different fields.
type TraceAndMetaKey struct {
	// Hash is not sent forward, but it is used as the primary key
	// to not aggregate difference traces.
	Hash libpf.TraceHash
	// comm and apmServiceName are provided by the eBPF programs
	Comm           string
	ApmServiceName string
	Pid            int64
	Tid            int64
	// Process name is retrieved from /proc/PID/comm
	ProcessName string
	// Executable path is retrieved from /proc/PID/exe
	ExecutablePath string

	// ExtraMeta stores extra meta info that may have been produced by a
	// `SampleAttrProducer` instance. May be nil.
	ExtraMeta any
}

// TraceEventsTree stores samples and their related metadata in a tree-like
// structure optimized for the OTel Profiling protocol representation.
type TraceEventsTree map[ContainerID]map[libpf.Origin]KeyToEventMapping

// ContainerID represents an extracted key from /proc/<PID>/cgroup.
type ContainerID string

// KeyToEventMapping supports temporary mapping traces to additional information.
type KeyToEventMapping map[TraceAndMetaKey]*TraceEvents
