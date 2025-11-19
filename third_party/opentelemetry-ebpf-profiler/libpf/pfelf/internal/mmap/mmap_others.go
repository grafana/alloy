//go:build !linux

// Package mmap is inspired by golang.org/x/exp/mmap with
// additional functionality.
package mmap // import "go.opentelemetry.io/ebpf-profiler/libpf/pfelf/internal/mmap"

func (r *ReaderAt) SetMadvDontNeed() error {
	return nil
}

func (r *ReaderAt) setRandom() error {
	return nil
}
