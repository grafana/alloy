// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/elastic/go-freelru"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/lpm"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
)

func TestJavaSymbolExtraction(t *testing.T) {
	id := hotspotData{}
	vmd, _ := id.GetOrInit(func() (hotspotVMData, error) {
		vmd := hotspotVMData{}
		vmd.vmStructs.Symbol.Length = 2
		vmd.vmStructs.Symbol.Body = 4
		return vmd, nil
	})

	maxLength := 1024
	sym := make([]byte, vmd.vmStructs.Symbol.Body+uint(maxLength))
	rd := bytes.NewReader(sym)
	rm := remotememory.RemoteMemory{ReaderAt: rd}

	addrToSymbol, err := freelru.New[libpf.Address, libpf.String](2, libpf.Address.Hash32)
	require.NoError(t, err, "symbol cache failed")

	ii := hotspotInstance{
		d:            &id,
		rm:           rm,
		addrToSymbol: addrToSymbol,
		prefixes:     libpf.Set[lpm.Prefix]{},
		stubs:        map[libpf.Address]StubRoutine{},
	}

	str := strings.Repeat("a", maxLength)
	copy(sym[vmd.vmStructs.Symbol.Body:], str)
	for i := 0; i <= maxLength; i++ {
		binary.LittleEndian.PutUint16(sym[vmd.vmStructs.Symbol.Length:], uint16(i))
		got := ii.getSymbol(0)
		assert.Equal(t, str[:i], got.String(), "symbol length %d mismatched read", i)
		ii.addrToSymbol.Purge()
	}
}
