// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package pfelf implements functions for processing of ELF files and extracting data from
// them. This file implements an interface to open ELF files from arbitrary location with name.

package pfelf // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf"

// ELFOpener is the interface to open ELF files from arbitrary location with given filename.
//
// Implementations must be safe to be called from different threads simultaneously.
type ELFOpener interface {
	OpenELF(string) (*File, error)
}

// SystemOpener implements ELFOpener by opening files from file system
type systemOpener struct{}

func (systemOpener) OpenELF(file string) (*File, error) {
	return Open(file)
}

var SystemOpener systemOpener
