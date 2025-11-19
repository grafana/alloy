// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interpreter // import "go.opentelemetry.io/ebpf-profiler/interpreter"

import (
	"errors"
	"unsafe"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/tpbase"
	"go.opentelemetry.io/ebpf-profiler/util"
)

const (
	// LruFunctionCacheSize is the LRU size for caching functions for an interpreter.
	// This should reflect the number of hot functions that are seen often in a trace.
	LruFunctionCacheSize = 1024
)

var (
	// UnknownSourceFile is the source file name to use when the real one is not available
	UnknownSourceFile = libpf.Intern("<unknown>")

	// AnonymousFunction is the name to be used when a function is anonymous.
	AnonymousFunctionName = libpf.Intern("<anonymous>")

	// TopLevelFunctionName is the name to be used when a function does not have a name,
	// but we can deduce that it is at the highest possible scope (e.g for top-level PHP code)
	TopLevelFunctionName = libpf.Intern("<top-level>")

	// UnknownFunction is the name to be used when a function name is not known.
	UnknownFunctionName = libpf.Intern("<unknown>")

	ErrMismatchInterpreterType = errors.New("mismatched interpreter type")
)

// The following function Loader and interfaces Data and Instance work together
// as an abstraction to support language specific eBPF unwinding and host agent side symbolization
// of frames.
//
// Functionality for these interfaces is divided as follows:
//
//  1. Loader is responsible for recognizing if the given mapping/ELF DSO matches by name,
//     and later by content, to an interpreter supported by the specific implementation.
//     If yes, it returns Data for this specific DSO. The Loader loads and checks data
//     from given ELF DSO. The intent is to load needed symbols and keep their addresses
//     relative to the file virtual address space. It can also load static data from the
//     DSO, such as the exact interpreter version string or number. All this is returned
//     in a data structure that implements Data interface.
//
//  2. Data is the interface to operate on per-ELF DSO data. ProcessManager receives this
//     interface from the Loader, and stores it in a map to cache them by FileID. That is,
//     each ELF DSO is probed by the Loaders only the first time it is seen. The returned
//     Data is then reused for all other processes using the same ELF DSO without need to
//     extract information from it by the loader.
//
//     The Attach method will populate the needed eBPF maps with the pre-parsed data from
//     the Data and PID specific AttachData. E.g. it can calculate the target memory addresses
//     by adding the file virtual address from cached Data and the process AttachData mapping
//     "bias". If additional per-PID structures need to be maintained it can instantiate new
//     Instance structures for those. Finally an Instance interface is returned: either to
//     the per-PID Instance structure, or if no per-PID data is kept, the main Data structure
//     can also implement this interface.
//
//  3. Instance is the interface to operate on per-PID data. This interface
//     is tracked by the ProcessManager by PID and the mapped-at-address.
//
//     The ProcessManager will delegate frame symbolization to this interface,
//     and it will also call this interface's Detach to clean up eBPF maps release any
//     per-PID resource held.
//
// The split of Data and Instance and the way the methods signatures are designed (passing ebpfMaps
// and pid) allows an interpreter implementation to keep just per-ELF information (when the xxxData
// implements both interfaces) or additionally track per-PID information (separate data types for
// the Data and Instance).
//
// Data (and Instance) should generally match one eBPF tracer implementation. However, it is
// possible to have several Loaders that would return same type of Data. For example,
// xxInterpreter 2 and xxInterpreter 3 can likely be unwound by the same unwinding strategy but
// perhaps the symbol names or the way to extract introspection data is different. Or perhaps we
// need to hard code different well known offsets in the xxData. It allows then to still
// share the Data and Instance code between these versions.

// EbpfHandler provides the functionality for interpreters to interact with eBPF maps.
type EbpfHandler interface {
	// UpdateInterpreterOffsets adds the given offsetRanges to the eBPF map interpreter_offsets.
	UpdateInterpreterOffsets(ebpfProgIndex uint16, fileID host.FileID,
		offsetRanges []util.Range) error

	// UpdateProcData adds the given interpreter data to the named eBPF map.
	UpdateProcData(typ libpf.InterpreterType, pid libpf.PID, data unsafe.Pointer) error

	// DeleteProcData removes any data from the named eBPF map.
	DeleteProcData(typ libpf.InterpreterType, pid libpf.PID) error

	// UpdatePidInterpreterMapping updates the eBPF map pid_page_to_mapping_info
	// to call given interpreter unwinder.
	UpdatePidInterpreterMapping(libpf.PID, lpm.Prefix, uint8, host.FileID, uint64) error

	// DeletePidInterpreterMapping removes the element specified by pid, prefix
	// rom the eBPF map pid_page_to_mapping_info.
	DeletePidInterpreterMapping(libpf.PID, lpm.Prefix) error
}

// Loader is a function to detect and load data from given interpreter ELF file.
// ProcessManager will call each configured Loader in order to see if additional handling and data
// is needed to unwind interpreter frames.
//
// A Loader can return one of the following value combinations:
//
//   - `nil, nil`, indicating that it didn't detect the interpreter to belong to it
//   - `data, nil`, indicating that it wants to handle the executable
//   - `nil, error`, indicating that a permanent failure occurred during interpreter detection
type Loader func(ebpf EbpfHandler, info *LoaderInfo) (Data, error)

// Data is the interface to operate on per-ELF DSO data.
type Data interface {
	// Attach checks if the given dso is supported, and loads the information
	// of it to the ebpf maps.
	Attach(ebpf EbpfHandler, pid libpf.PID, bias libpf.Address, rm remotememory.RemoteMemory) (
		Instance, error)

	// Unload can undo any allocations or eBPF entries the Loader function created
	Unload(ebpf EbpfHandler)
}

// Instance is the interface to operate on per-PID data.
type Instance interface {
	// Detach removes any information from the ebpf maps. The pid is given as argument so
	// simple interpreters can use the global Data also as the Instance implementation.
	Detach(ebpf EbpfHandler, pid libpf.PID) error

	// SynchronizeMappings is called when the processmanager has reread process memory
	// mappings. Interpreters not needing to process these events can simply ignore them
	// by just returning a nil.
	SynchronizeMappings(ebpf EbpfHandler, exeReporter reporter.ExecutableReporter,
		pr process.Process, mappings []process.Mapping) error

	// UpdateTSDInfo is called when the process C-library Thread Specific Data related
	// introspection data has been updated.
	UpdateTSDInfo(ebpf EbpfHandler, pid libpf.PID, info tpbase.TSDInfo) error

	// Symbolize converts one ebpf frame to one or more (if inlining was expanded) libpf.Frame.
	// The resulting libpf.Frame values are appended to frames.
	Symbolize(ebpfFrame *host.Frame, frames *libpf.Frames) error

	// GetAndResetMetrics collects the metrics from the Instance and resets
	// the counters to their initial value.
	GetAndResetMetrics() ([]metrics.Metric, error)

	// Release resources that are used to symbolize a stack.
	ReleaseResources() error
}
