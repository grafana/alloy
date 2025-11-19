// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracer // import "go.opentelemetry.io/ebpf-profiler/tracer"

import (
	cebpf "github.com/cilium/ebpf"
)

// Make accessible for testing
func (t *Tracer) GetEbpfMaps() map[string]*cebpf.Map {
	return t.ebpfMaps
}
