// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo // import "go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
)

// ELFStackDeltaProvider extracts stack deltas from ELF executables available
// via the pfelf.File interface.
type ELFStackDeltaProvider struct {
	// Metrics
	successCount         atomic.Uint64
	extractionErrorCount atomic.Uint64
}

// Compile time check that the ELFStackDeltaProvider implements its interface correctly.
var _ nativeunwind.StackDeltaProvider = (*ELFStackDeltaProvider)(nil)

// NewStackDeltaProvider creates a stack delta provider using the ELF eh_frame extraction.
func NewStackDeltaProvider() nativeunwind.StackDeltaProvider {
	return &ELFStackDeltaProvider{}
}

// GetIntervalStructuresForFile builds the stack delta information for a single executable.
func (provider *ELFStackDeltaProvider) GetIntervalStructuresForFile(elfRef *pfelf.Reference,
	interval *sdtypes.IntervalData) error {
	err := ExtractELF(elfRef, interval)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			provider.extractionErrorCount.Add(1)
		}
		return fmt.Errorf("failed to extract stack deltas from %s: %w",
			elfRef.FileName(), err)
	}
	provider.successCount.Add(1)
	return nil
}

func (provider *ELFStackDeltaProvider) GetAndResetStatistics() nativeunwind.Statistics {
	return nativeunwind.Statistics{
		Success:          provider.successCount.Swap(0),
		ExtractionErrors: provider.extractionErrorCount.Swap(0),
	}
}
