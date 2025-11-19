// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package pfelf implements functions for processing of ELF files and extracting data from
// them. This file implements Reference which opens and caches a File on demand.

package pfelf // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf"

// Reference is a reference to an ELF file which is loaded and cached on demand.
type Reference struct {
	// Interface to open ELF files as needed
	ELFOpener

	// fileName is the full path of the ELF to open.
	fileName string

	// elfFile contains the cached ELF file
	elfFile *File
}

// NewReference returns a new Reference
func NewReference(fileName string, elfOpener ELFOpener) *Reference {
	return &Reference{fileName: fileName, ELFOpener: elfOpener}
}

// FileName returns the file name associated with this Reference
func (ref *Reference) FileName() string {
	return ref.fileName
}

// GetELF returns the File to access this File and keeps it cached. The
// caller of this functions must not Close the File.
func (ref *Reference) GetELF() (*File, error) {
	var err error
	if ref.elfFile == nil {
		ref.elfFile, err = ref.OpenELF(ref.fileName)
	}
	return ref.elfFile, err
}

// Close closes the File if it has been opened earlier.
func (ref *Reference) Close() {
	if ref.elfFile != nil {
		_ = ref.elfFile.Close()
		ref.elfFile = nil
	}
}
