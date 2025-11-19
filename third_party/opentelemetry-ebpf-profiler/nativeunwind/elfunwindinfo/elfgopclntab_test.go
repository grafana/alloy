// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo

import (
	"debug/elf"
	"testing"

	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Go 1.2 spec Appendix: PC-Value Table Encoding example
func TestPcval(t *testing.T) {
	res := []struct {
		val int32
		pc  uint
	}{
		{0, 0x2019},
		{32, 0x206b},
		{40, 0x206d},
		{48, 0x2073},
		{40, 0x2074},
		{32, 0x209b},
		{0, 0x209c},
	}
	data := []byte{
		0x02, 0x19, 0x40, 0x52, 0x10, 0x02, 0x10, 0x06,
		0x0f, 0x01, 0x0f, 0x27, 0x3f, 0x01, 0x00}
	p := newPcval(data, 0x2000, 1)
	i := 0
	for ok := true; ok; ok = p.step() {
		t.Logf("Pcval %d, %x", p.val, p.pcEnd)
		assert.Equal(t, res[i].val, p.val)
		assert.Equal(t, res[i].pc, p.pcEnd)
		i++
	}
	assert.Equal(t, len(res), i)
}

// Pcval with sequence that would result in out-of-bound read
func TestPcvalInvalid(_ *testing.T) {
	data := []byte{0x81}
	p := newPcval(data, 0x2000, 1)
	for p.step() {
	}
}

// Some strategy tests
func TestGoStrategy(t *testing.T) {
	res := []struct {
		file   string
		result strategy
	}{
		{"foo.go", strategyFramePointer},
		{"foo.s", strategyDeltasWithoutFrame},
		{"go/src/crypto/elliptic/p256_asm.go", strategyDeltasWithFrame},
	}
	for _, x := range res {
		s := getSourceFileStrategy(elf.EM_X86_64, x.file, strategyFramePointer)
		assert.Equal(t, x.result, s)
	}
}

func TestParseGoPclntab(t *testing.T) {
	tests := map[string]struct {
		elfFile string
	}{
		// helloworld is a very basic Go binary without special build flags.
		"regular Go binary":       {elfFile: "testdata/helloworld"},
		"regular ARM64 Go binary": {elfFile: "testdata/helloworld.arm64"},
		// helloworld.pie is a Go binary that is build with PIE enabled.
		"PIE Go binary": {elfFile: "testdata/helloworld.pie"},
		// helloworld.stripped.pie is a Go binary that is build with PIE enabled and all debug
		// information stripped.
		"stripped PIE Go binary": {elfFile: "testdata/helloworld.stripped.pie"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ef, err := pfelf.Open(test.elfFile)
			require.NoError(t, err)

			ee := elfExtractor{
				file:   ef,
				hooks:  &extractionFilter{},
				deltas: &sdtypes.StackDeltaArray{},
			}
			err = ee.parseGoPclntab()
			require.NoError(t, err)
			assert.NotEmpty(t, *ee.deltas)
		})
	}
}
