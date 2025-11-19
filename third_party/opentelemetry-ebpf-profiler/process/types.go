// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// This file defines the interface to access a Process state.

package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"debug/elf"
	"io"
	"strings"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/util"
)

// VdsoPathName is the path to use for VDSO mappings.
var VdsoPathName = libpf.Intern("linux-vdso.1.so")

// vdsoInode is the synthesized inode number for VDSO mappings.
const vdsoInode = 50

// Mapping contains information about a memory mapping.
type Mapping struct {
	// Vaddr is the virtual memory start for this mapping.
	Vaddr uint64
	// Length is the length of the mapping.
	Length uint64
	// Flags contains the mapping flags and permissions.
	Flags elf.ProgFlag
	// FileOffset contains for file backed mappings the offset from the file start.
	FileOffset uint64
	// Device holds the device ID where the file is located.
	Device uint64
	// Inode holds the mapped file's inode number.
	Inode uint64
	// Path contains the file name for file backed mappings.
	Path libpf.String
}

func (m *Mapping) IsExecutable() bool {
	return m.Flags&elf.PF_X == elf.PF_X
}

func (m *Mapping) IsAnonymous() bool {
	return m.Path == libpf.NullString || m.IsMemFD()
}

func (m *Mapping) IsMemFD() bool {
	return strings.HasPrefix(m.Path.String(), "/memfd:")
}

func (m *Mapping) IsVDSO() bool {
	return m.Path == VdsoPathName
}

func (m *Mapping) GetOnDiskFileIdentifier() util.OnDiskFileIdentifier {
	return util.OnDiskFileIdentifier{
		DeviceID: m.Device,
		InodeNum: m.Inode,
	}
}

// ThreadInfo contains the information about a thread CPU state needed for unwinding.
type ThreadInfo struct {
	// TPBase contains the Thread Pointer Base value.
	TPBase uint64
	// GPRegs contains the CPU state (registers) for the thread.
	GPRegs []byte
	// LWP is the Light Weight Process ID (thread ID).
	LWP uint32
}

// MachineData contains machine specific information about the process.
type MachineData struct {
	// Machine is the Process Machine type.
	Machine elf.Machine
	// CodePACMask contains the PAC mask for code pointers. ARM64 specific, otherwise 0.
	CodePACMask uint64
	// DataPACMask contains the PAC mask for data pointers. ARM64 specific, otherwise 0.
	DataPACMask uint64
}

// ReadAtCloser combines the io.ReaderAt and io.Closer interfaces.
type ReadAtCloser interface {
	io.ReaderAt
	io.Closer
}

// MetaConfig provides options that influences gathering ProcessMeta.
type MetaConfig struct {
	// IncludeEnvVars holds a list of env vars that should be captured from the process.
	IncludeEnvVars libpf.Set[string]
}

// ProcessMeta contains metadata about a tracked process.
type ProcessMeta struct {
	// process name retrieved from /proc/PID/comm
	Name string
	// executable path retrieved from /proc/PID/exe
	Executable string
	// process env vars from /proc/PID/environ
	EnvVariables map[string]string
	// container ID retrieved from /proc/PID/cgroup
	ContainerID string
}

// Process is the interface to inspect ELF coredump/process.
// The current implementations do not allow concurrent access to this interface
// from different goroutines. As an exception the ELFOpener and the returned
// GetRemoteMemory object are safe for concurrent use.
type Process interface {
	// PID returns the process identifier.
	PID() libpf.PID

	// GetMachineData reads machine specific data from the target process.
	GetMachineData() MachineData

	// GetProcessMeta returns process specific metadata.
	GetProcessMeta(MetaConfig) ProcessMeta

	// GetExe returns the executable path of the process.
	GetExe() (string, error)

	// GetMappings reads and parses process memory mappings.
	GetMappings() ([]Mapping, uint32, error)

	// GetThreads reads the process thread states.
	GetThreads() ([]ThreadInfo, error)

	// GetRemoteMemory returns a remote memory reader accessing the target process.
	GetRemoteMemory() remotememory.RemoteMemory

	// OpenMappingFile returns ReadAtCloser accessing the backing file of the mapping.
	OpenMappingFile(*Mapping) (ReadAtCloser, error)

	// GetMappingFileLastModifed returns the timestamp when the backing file was last modified
	// or zero if an error occurs or mapping file is not accessible via filesystem.
	GetMappingFileLastModified(*Mapping) int64

	// CalculateMappingFileID calculates FileID of the backing file.
	CalculateMappingFileID(*Mapping) (libpf.FileID, error)

	// ExtractAsFile returns a filename suitable for opening the given file from
	// the target process namespace. This is a last resort method to access the file
	// when the ReaderAt interface from OpenMappingFile is not sufficient. The returned
	// filename may refer to /proc or be a temporarily file, and it must not be modified
	// or deleted.
	ExtractAsFile(string) (string, error)

	io.Closer

	pfelf.ELFOpener
}
