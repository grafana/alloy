// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"

	log "github.com/sirupsen/logrus"
)

type StoreCoredump struct {
	*process.CoredumpProcess

	store     *modulestore.Store
	modules   map[string]ModuleInfo
	tempFiles map[string]string
}

var _ pfelf.ELFOpener = &StoreCoredump{}

func (scd *StoreCoredump) openFile(path string) (process.ReadAtCloser, error) {
	info, ok := scd.modules[path]
	if !ok {
		// The test case creator should have bundled everything.
		// However, old test cases have no bundle at all, so give a warning
		// only if some modules exists.
		if len(scd.modules) != 0 {
			log.Warnf("Store does not bundle %s", path)
		}
		return nil, fmt.Errorf("failed to open file `%s`: %w", path, os.ErrNotExist)
	}

	// The module is available from store.
	file, err := scd.store.OpenBufferedReadAt(info.Ref, 4*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("failed to open file `%s`: %w", path, err)
	}
	return file, nil
}

func (scd *StoreCoredump) OpenMappingFile(m *process.Mapping) (process.ReadAtCloser, error) {
	return scd.openFile(m.Path.String())
}

func (scd *StoreCoredump) OpenELF(path string) (*pfelf.File, error) {
	file, err := scd.openFile(path)
	if err == nil {
		return pfelf.NewFile(file, 0, false)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// Fallback to the native CoredumpProcess
	return scd.CoredumpProcess.OpenELF(path)
}

func (scd *StoreCoredump) ExtractAsFile(file string) (string, error) {
	info, ok := scd.modules[file]
	if !ok {
		return "", os.ErrNotExist
	}

	f, err := os.CreateTemp("", "ebpf-profiler-coredump.*")
	if err != nil {
		return "", err
	}
	tmpFile := f.Name()
	_ = f.Close()

	if err := scd.store.UnpackModuleToPath(info.Ref, tmpFile); err != nil {
		_ = os.Remove(tmpFile)
		return "", err
	}
	scd.tempFiles[file] = tmpFile
	return tmpFile, nil
}

func (scd *StoreCoredump) Close() error {
	for _, tmpFile := range scd.tempFiles {
		_ = os.Remove(tmpFile)
	}
	return scd.CoredumpProcess.Close()
}

func OpenStoreCoredump(store *modulestore.Store, coreFileRef modulestore.ID, modules []ModuleInfo) (
	process.Process, error) {
	// Open the coredump from the module store.
	reader, err := store.OpenBufferedReadAt(coreFileRef, 16*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("failed to open coredump file reader: %w", err)
	}
	coreELF, err := pfelf.NewFile(reader, 0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to open coredump ELF: %w", err)
	}
	core, err := process.OpenCoredumpFile(coreELF)
	if err != nil {
		return nil, fmt.Errorf("failed to open coredump: %w", err)
	}

	moduleMap := map[string]ModuleInfo{}
	for _, module := range modules {
		moduleMap[module.LocalPath] = module
	}

	return &StoreCoredump{
		CoredumpProcess: core,

		store:     store,
		modules:   moduleMap,
		tempFiles: make(map[string]string),
	}, nil
}
