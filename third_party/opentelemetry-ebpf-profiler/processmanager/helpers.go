// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processmanager // import "go.opentelemetry.io/ebpf-profiler/processmanager"

import (
	lru "github.com/elastic/go-freelru"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter"
)

type lruFileIDMapper struct {
	cache *lru.SyncedLRU[host.FileID, libpf.FrameMappingFile]
}

// identityHash maps the host.FileID to a 32bit value.
// No need to explicitly hash the FileID, since it's already a hash value.
func identityHash(key host.FileID) uint32 {
	return uint32(key)
}

func newFileIDMapper(size int) (*lruFileIDMapper, error) {
	cache, err := lru.NewSynced[host.FileID, libpf.FrameMappingFile](uint32(size), identityHash)
	if err != nil {
		return nil, err
	}
	return &lruFileIDMapper{cache}, nil
}

func (fm *lruFileIDMapper) Get(key host.FileID) (libpf.FrameMappingFile, bool) {
	if mappingFile, ok := fm.cache.Get(key); ok {
		return mappingFile, true
	}

	log.Warnf("Failed to lookup file ID %#x", key)
	return libpf.FrameMappingFile{}, false
}

func (fm *lruFileIDMapper) Set(key host.FileID, val libpf.FrameMappingFile) {
	fm.cache.Add(key, val)
	log.Debugf("Stored file ID mapping %#x -> %#x", key, val.Value())
}

var _ FileIDMapper = (*lruFileIDMapper)(nil)

// MapFileIDMapper implements the FileIDMApper using a map (for testing)
type MapFileIDMapper struct {
	fileMap map[host.FileID]libpf.FrameMappingFile
}

func NewMapFileIDMapper() *MapFileIDMapper {
	return &MapFileIDMapper{
		fileMap: make(map[host.FileID]libpf.FrameMappingFile),
	}
}

func (fm *MapFileIDMapper) Get(key host.FileID) (libpf.FrameMappingFile, bool) {
	if value, ok := fm.fileMap[key]; ok {
		return value, true
	}
	return libpf.FrameMappingFile{}, true
}

func (fm *MapFileIDMapper) Set(key host.FileID, value libpf.FrameMappingFile) {
	fm.fileMap[key] = value
}

var _ FileIDMapper = (*MapFileIDMapper)(nil)

// FileIDMapper is responsible for mapping between 64-bit file IDs to the frame mapping metadata.
type FileIDMapper interface {
	// Retrieve the metadata for given 64-bit file ID.
	Get(fileID host.FileID) (libpf.FrameMappingFile, bool)
	// Associate the metadata for given 64-bit file ID.
	Set(fileID host.FileID, metadata libpf.FrameMappingFile)
}

// executableReporterStub is a stub to implement reporter.ExecutableReporter which is used
// as the reporter by default. This can be overridden on at processmanager creation time.
type executableReporterStub struct {
}

// ReportExecutable satisfies the reporter.ExecutableReporter interface.
func (er executableReporterStub) ReportExecutable(args *reporter.ExecutableMetadata) {
}

var _ reporter.ExecutableReporter = executableReporterStub{}
