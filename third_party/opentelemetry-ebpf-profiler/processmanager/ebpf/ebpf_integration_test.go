//go:build integration && linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"testing"

	cebpf "github.com/cilium/ebpf"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/rlimit"
	"go.opentelemetry.io/ebpf-profiler/support"
)

func loadTracers(t *testing.T) *ebpfMapsImpl {
	t.Helper()

	coll, err := support.LoadCollectionSpec()
	require.NoError(t, err)

	restoreRlimit, err := rlimit.MaximizeMemlock()
	require.NoError(t, err)
	defer restoreRlimit()

	pidPageToMappingInfo, err := cebpf.NewMap(coll.Maps["pid_page_to_mapping_info"])
	require.NoError(t, err)

	return &ebpfMapsImpl{
		PidPageToMappingInfo: pidPageToMappingInfo,
	}
}

func TestLPM(t *testing.T) {
	tests := map[string]struct {
		pid      libpf.PID
		page     uint64
		pageBits uint32
		rip      uint64
		fileID   uint64
		bias     uint64
	}{
		"direct": {pid: 1000, page: 0xAA55AA, pageBits: 64, rip: 0xAA55AA, fileID: 123, bias: 456},
		"random": {pid: 123, page: 0x500000, pageBits: 44, rip: 0x5a63b5, fileID: 456, bias: 789},
	}

	impl := loadTracers(t)

	for name, test := range tests {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			prefix := lpm.Prefix{
				Key:    test.page,
				Length: test.pageBits,
			}
			err := impl.UpdatePidPageMappingInfo(test.pid, prefix, test.fileID, test.bias)
			require.NoError(t, err)

			fileID, bias, err := impl.LookupPidPageInformation(test.pid, test.rip)
			if assert.NoError(t, err) {
				assert.Equal(t, test.fileID, uint64(fileID))
				assert.Equal(t, test.bias, bias)
			}

			_, err = impl.DeletePidPageMappingInfo(test.pid, []lpm.Prefix{prefix})
			require.NoError(t, err)
		})
	}
}

func TestBatchOperations(t *testing.T) {
	for _, mapType := range []cebpf.MapType{cebpf.Hash, cebpf.Array, cebpf.LPMTrie} {
		t.Run(mapType.String(), func(t *testing.T) {
			err := probeBatchOperations(mapType)
			if err != nil {
				require.ErrorIs(t, err, cebpf.ErrNotSupported)
			}
		})
	}
}
