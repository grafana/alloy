// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processmanager // import "go.opentelemetry.io/ebpf-profiler/processmanager"

import (
	"sync"
	"sync/atomic"

	lru "github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/process"
	pmebpf "go.opentelemetry.io/ebpf-profiler/processmanager/ebpfapi"
	eim "go.opentelemetry.io/ebpf-profiler/processmanager/execinfomanager"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/times"
	"go.opentelemetry.io/ebpf-profiler/tpbase"
	"go.opentelemetry.io/ebpf-profiler/util"
)

// elfInfo contains cached data from an executable needed for processing mappings.
// A negative cache entry may also be recorded with err set to indicate permanent
// error. This avoids inspection of non-ELF or corrupted files again and again.
type elfInfo struct {
	err           error
	lastModified  int64
	mappingFile   libpf.FrameMappingFile
	addressMapper pfelf.AddressMapper
}

// frameCacheKey is the LRU cache key for caching frames.
type frameCacheKey struct {
	host.Frame
	libpf.PID
}

// ProcessManager is responsible for managing the events happening throughout the lifespan of a
// process.
type ProcessManager struct {
	// A mutex to synchronize access to internal data within this struct.
	mu sync.RWMutex

	// interpreterTracerEnabled indicates if at last one non-native tracer is loaded.
	interpreterTracerEnabled bool

	// eim stores per executable (file ID) information.
	eim *eim.ExecutableInfoManager

	// interpreters records the interpreter.Instance interface which contains hooks for
	// process exits, and various other situations needing interpreter specific attention.
	// The key of the first map is a process ID, while the key of the second map is
	// the unique on-disk identifier of the interpreter DSO.
	interpreters map[libpf.PID]map[util.OnDiskFileIdentifier]interpreter.Instance

	// pidToProcessInfo keeps track of the executable memory mappings.
	pidToProcessInfo map[libpf.PID]*processInfo

	// exitEvents records the pid exit time and is a list of pending exit events to be handled.
	exitEvents map[libpf.PID]times.KTime

	// ebpf contains the interface to manipulate ebpf maps
	ebpf pmebpf.EbpfHandler

	// FileIDMapper provides a cache that implements the FileIDMapper interface. The tracer writes
	// the 64-bit to 128-bit file ID mapping to the cache, as this is where the two values are
	// created. The attached interpreters read from the cache when converting traces prior to
	// sending to the collection agent. The cache resides in this package instead of the ebpf
	// package to prevent circular imports.
	FileIDMapper FileIDMapper

	// elfInfoCacheHit
	elfInfoCacheHit  atomic.Uint64
	elfInfoCacheMiss atomic.Uint64

	// frame conversion
	frameCacheHit  atomic.Uint64
	frameCacheMiss atomic.Uint64

	// mappingStats are statistics for parsing process mappings
	mappingStats struct {
		errProcNotExist    atomic.Uint32
		errProcESRCH       atomic.Uint32
		errProcPerm        atomic.Uint32
		numProcAttempts    atomic.Uint32
		maxProcParseUsec   atomic.Uint32
		totalProcParseUsec atomic.Uint32
		numProcParseErrors atomic.Uint32
	}

	// elfInfoCache provides a cache to quickly retrieve the ELF info and fileID for a particular
	// executable. It caches results based on iNode number and device ID. Locked LRU.
	elfInfoCache *lru.LRU[util.OnDiskFileIdentifier, elfInfo]

	// frameCache stores mappings from BPF frame to the symbolized frames.
	// This allows avoiding the overhead of re-doing user-mode symbolization
	// of frames that we have recently seen already.
	frameCache *lru.LRU[frameCacheKey, libpf.Frames]

	// traceReporter is the interface to report traces
	traceReporter reporter.TraceReporter

	// exeReporter is the interface to report executables
	exeReporter reporter.ExecutableReporter

	// Reporting function which is used to report information to our backend.
	metricsAddSlice func([]metrics.Metric)

	// pidPageToMappingInfoSize reflects the current size of the eBPF hash map
	// pid_page_to_mapping_info.
	pidPageToMappingInfoSize uint64

	// filterErrorFrames determines whether error frames are dropped by `ConvertTrace`.
	filterErrorFrames bool

	policy dynamicprofiling.Policy

	// includeEnvVars holds a list of env vars that should be captured from processes
	includeEnvVars libpf.Set[string]
}

// Mapping represents an executable memory mapping of a process.
type Mapping struct {
	// FileID represents the host-wide unique identifier of the mapped file.
	FileID host.FileID

	// Vaddr represents the starting virtual address of the mapping.
	Vaddr libpf.Address

	// Bias is the offset between the ELF on-disk virtual address space and the
	// virtual address where it is actually mapped in the process. Thus it is the
	// virtual address bias or "ASLR offset". It serves as a translation offset
	// from the process VA space into the VA space of the ELF file. It's calculated as
	// `bias = vaddr_in_proc - vaddr_in_elf`.
	// Adding the bias to a VA in ELF space translates it into process space.
	Bias uint64

	// Length represents the memory size of the mapping.
	Length uint64

	// Device number of the backing file
	Device uint64

	// Inode number of the backing file
	Inode uint64

	// File offset of the backing file
	FileOffset uint64

	FilePath libpf.String
}

// GetOnDiskFileIdentifier returns the OnDiskFileIdentifier for the mapping
func (m *Mapping) GetOnDiskFileIdentifier() util.OnDiskFileIdentifier {
	return util.OnDiskFileIdentifier{
		DeviceID: m.Device,
		InodeNum: m.Inode,
	}
}

// processInfo contains information about the executable mappings
// and Thread Specific Data of a process.
type processInfo struct {
	// process metadata, fixed for process lifetime (read-only)
	meta process.ProcessMeta
	// executable mappings keyed by start address.
	mappings map[libpf.Address]*Mapping
	// executable mappings keyed by host file ID.
	mappingsByFileID map[host.FileID]map[libpf.Address]*Mapping
	// C-library Thread Specific Data information
	tsdInfo *tpbase.TSDInfo
}

// addMapping adds a mapping to the internal indices.
func (pi *processInfo) addMapping(m Mapping) {
	p := &m
	pi.mappings[m.Vaddr] = p

	inner := pi.mappingsByFileID[m.FileID]
	if inner == nil {
		inner = make(map[libpf.Address]*Mapping, 1)
		pi.mappingsByFileID[m.FileID] = inner
	}
	inner[m.Vaddr] = p
}

// removeMapping removes a mapping from the internal indices.
func (pi *processInfo) removeMapping(m *Mapping) {
	delete(pi.mappings, m.Vaddr)

	if inner, ok := pi.mappingsByFileID[m.FileID]; ok {
		delete(inner, m.Vaddr)
		if len(inner) != 0 {
			delete(pi.mappingsByFileID, m.FileID)
		}
	}
}
