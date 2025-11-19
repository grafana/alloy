//go:build linux

// Package mmap is inspired by golang.org/x/exp/mmap with
// additional functionality.
package mmap // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf/internal/mmap"

import "syscall"

func (r *ReaderAt) SetMadvDontNeed() error {
	return syscall.Madvise(r.data, syscall.MADV_DONTNEED)
}

func (r *ReaderAt) setRandom() error {
	return syscall.Madvise(r.data, syscall.MADV_RANDOM)
}
