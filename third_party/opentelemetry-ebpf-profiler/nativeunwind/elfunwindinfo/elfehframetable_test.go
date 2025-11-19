// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

func TestLookupFDE(t *testing.T) {
	checks := []struct {
		at       uintptr
		expected FDE
	}{
		{at: 0x0, expected: FDE{}},
		{at: 0x840, expected: FDE{}},
		{at: 0x850, expected: FDE{PCBegin: 0x850, PCRange: 0x10}},
		{at: 0x855, expected: FDE{PCBegin: 0x850, PCRange: 0x10}},
		{at: 0x859, expected: FDE{PCBegin: 0x850, PCRange: 0x10}},
		{at: 0x860, expected: FDE{PCBegin: 0x860, PCRange: 0x68}},
		{at: 0x865, expected: FDE{PCBegin: 0x860, PCRange: 0x68}},
		{at: 0x8c7, expected: FDE{PCBegin: 0x860, PCRange: 0x68}},
		{at: 0x8c8, expected: FDE{}},
		{at: 0x8c9, expected: FDE{}},
		{at: 0x8cf, expected: FDE{}},
		{at: 0x8d0, expected: FDE{PCBegin: 0x8d0, PCRange: 0x11f}},
		{at: 0x8d3, expected: FDE{PCBegin: 0x8d0, PCRange: 0x11f}},
		{at: 0x9ee, expected: FDE{PCBegin: 0x8d0, PCRange: 0x11f}},
		{at: 0x9ef, expected: FDE{}},
		{at: 0x9f0, expected: FDE{PCBegin: 0x9f0, PCRange: 0x2b}},
		{at: 0x9f1, expected: FDE{PCBegin: 0x9f0, PCRange: 0x2b}},
		{at: 0xa1a, expected: FDE{PCBegin: 0x9f0, PCRange: 0x2b}},
		{at: 0xa1b, expected: FDE{}},
		{at: 0xa1c, expected: FDE{}},
		{at: 0xb1f, expected: FDE{}},
		{at: 0xb20, expected: FDE{PCBegin: 0xb20, PCRange: 0x65}},
		{at: 0xb32, expected: FDE{PCBegin: 0xb20, PCRange: 0x65}},
		{at: 0xb84, expected: FDE{PCBegin: 0xb20, PCRange: 0x65}},
		{at: 0xb85, expected: FDE{}},
		{at: 0xb90, expected: FDE{PCBegin: 0xb90, PCRange: 0x2}},
		{at: 0xb91, expected: FDE{PCBegin: 0xb90, PCRange: 0x2}},
		{at: 0xb92, expected: FDE{}},
		{at: 0xb93, expected: FDE{}},
		{at: 0x1000, expected: FDE{}},
		{at: 0xcafe000, expected: FDE{}},
	}
	elf, err := getUsrBinPfelf()
	require.NoError(t, err)
	t.Cleanup(func() {
		err = elf.Close()
		require.NoError(t, err)
	})
	e, err := NewEhFrameTable(elf)
	require.NoError(t, err)
	for _, check := range checks {
		t.Run(fmt.Sprintf("%x", check.at), func(t *testing.T) {
			actual, err := e.LookupFDE(libpf.Address(check.at))
			if check.expected == (FDE{}) {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, check.expected, actual)
			}
		})
	}
}
