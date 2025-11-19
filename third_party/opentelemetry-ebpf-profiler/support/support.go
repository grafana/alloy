// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package support // import "go.opentelemetry.io/ebpf-profiler/support"

import (
	"bytes"

	cebpf "github.com/cilium/ebpf"
)

// LoadCollectionSpec is a wrapper around ebpf.LoadCollectionSpecFromReader and loads the eBPF
// Spec from the embedded file.
// We expect tracerData to hold all possible eBPF maps and programs.
func LoadCollectionSpec() (*cebpf.CollectionSpec, error) {
	return cebpf.LoadCollectionSpecFromReader(bytes.NewReader(tracerData))
}
