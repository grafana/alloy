//go:build !arm64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pacmask // import "go.opentelemetry.io/ebpf-profiler/pacmask"

// GetPACMask always returns 0 on this platform.
func GetPACMask() uint64 {
	return 0
}
