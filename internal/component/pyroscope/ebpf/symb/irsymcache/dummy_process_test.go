//go:build unix

package irsymcache

import (
	"errors"
	"os"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
)

// dummyProcess implements pfelf.Process for testing purposes
type dummyProcess struct {
	pid           libpf.PID
	mappings      []process.RawMapping
	mappingsError error
}

func (d *dummyProcess) GetProcessMeta(config process.MetaConfig) process.ProcessMeta {
	panic("implement me")
}

func (d *dummyProcess) GetExe() (libpf.String, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dummyProcess) PID() libpf.PID {
	return d.pid
}

func (d *dummyProcess) GetMachineData() process.MachineData {
	return process.MachineData{}
}

func (d *dummyProcess) IterateMappings(callback func(m process.RawMapping) bool) (uint32, error) {
	for _, m := range d.mappings {
		if !callback(m) {
			break
		}
	}
	return 0, d.mappingsError
}

func (d *dummyProcess) GetThreads() ([]process.ThreadInfo, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyProcess) GetRemoteMemory() remotememory.RemoteMemory {
	return remotememory.RemoteMemory{}
}

func (d *dummyProcess) GetMappingFileLastModified(_ *process.RawMapping) int64 {
	return 0
}

func (d *dummyProcess) CalculateMappingFileID(m *process.RawMapping) (libpf.FileID, error) {
	return libpf.FileIDFromExecutableFile(m.Path)
}

func (d *dummyProcess) OpenMappingFile(m *process.RawMapping) (process.ReadAtCloser, error) {
	return os.Open(m.Path)
}

func (d *dummyProcess) OpenELF(name string) (*pfelf.File, error) {
	return pfelf.Open(name)
}

func (d *dummyProcess) Close() error {
	return nil
}
