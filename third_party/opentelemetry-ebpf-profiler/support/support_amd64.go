//go:build amd64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package support // import "go.opentelemetry.io/ebpf-profiler/support"

import (
	_ "embed"
)

//go:embed ebpf/tracer.ebpf.amd64
var tracerData []byte
