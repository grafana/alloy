//go:build !arm64 && !amd64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package support // import "go.opentelemetry.io/ebpf-profiler/support"

// support_dummy.go satisfies build requirements where the eBPF tracers file does
// not exist.

var tracerData []byte
