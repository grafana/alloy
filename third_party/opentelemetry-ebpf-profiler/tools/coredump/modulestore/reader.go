// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package modulestore // import "go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"

import "io"

// ModuleReader allows reading a module from the module store.
type ModuleReader struct {
	io.ReaderAt
	io.Closer
	preferredReadSize uint
	size              uint
}

// PreferredReadSize returns the preferred size and alignment of reads on this reader.
func (m *ModuleReader) PreferredReadSize() uint {
	return m.preferredReadSize
}

// Size returns the uncompressed size of the module.
func (m *ModuleReader) Size() uint {
	return m.size
}
