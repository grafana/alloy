//go:build !(linux && (arm64 || amd64))

package reporter

// This file provides a no-op stub for platforms that don't support eBPF profiling.
// The actual implementation is in pyroscope_uploader.go with linux build tags.
