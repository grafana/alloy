// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pfelf_test

import (
	"debug/elf"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/testsupport"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
)

var (
	// An ELF without DWARF symbols
	withoutDebugSymsPath = "testdata/without-debug-syms"
	// An ELF with DWARF symbols
	withDebugSymsPath = "testdata/with-debug-syms"
	// An ELF with only DWARF symbols
	separateDebugFile = "testdata/separate-debug-file"
)

func getELF(path string, t *testing.T) *elf.File {
	file, err := elf.Open(path)
	assert.NoError(t, err)
	return file
}

func TestGetBuildID(t *testing.T) {
	debugExePath, err := testsupport.WriteTestExecutable1()
	require.NoError(t, err)
	defer os.Remove(debugExePath)

	elfFile := getELF(debugExePath, t)
	defer elfFile.Close()

	buildID, err := pfelf.GetBuildID(elfFile)
	require.NoError(t, err)
	assert.Equal(t, "6920fd217a8416131f4377ef018a2c932f311b6d", buildID)
}

func TestGetDebugLink(t *testing.T) {
	debugExePath, err := testsupport.WriteTestExecutable1()
	require.NoError(t, err)
	defer os.Remove(debugExePath)

	elfFile := getELF(debugExePath, t)
	defer elfFile.Close()

	debugLink, crc32, err := pfelf.GetDebugLink(elfFile)
	require.NoError(t, err)
	assert.Equal(t, "dumpmscat-4.10.8-0.fc30.x86_64.debug", debugLink)
	assert.Equal(t, uint32(0xfe3099b8), uint32(crc32))
}

func TestGetBuildIDError(t *testing.T) {
	debugExePath, err := testsupport.WriteTestExecutable2()
	require.NoError(t, err)
	defer os.Remove(debugExePath)

	elfFile := getELF(debugExePath, t)
	defer elfFile.Close()

	buildID, err := pfelf.GetBuildID(elfFile)
	if assert.ErrorIs(t, pfelf.ErrNoBuildID, err) {
		assert.Empty(t, buildID)
	}
}

func TestGetDebugLinkError(t *testing.T) {
	debugExePath, err := testsupport.WriteTestExecutable2()
	require.NoError(t, err)
	defer os.Remove(debugExePath)

	elfFile := getELF(debugExePath, t)
	defer elfFile.Close()

	debugLink, _, err := pfelf.GetDebugLink(elfFile)
	if assert.ErrorIs(t, pfelf.ErrNoDebugLink, err) {
		assert.Empty(t, debugLink)
	}
}

func TestHasDWARFData(t *testing.T) {
	tests := map[string]struct {
		filePath       string
		expectedResult bool
	}{
		"ELF executable - no DWARF":   {withoutDebugSymsPath, false},
		"ELF executable - with DWARF": {withDebugSymsPath, true},
		"Separate debug symbols":      {separateDebugFile, true},
	}

	for testName, testCase := range tests {
		name := testName
		tc := testCase
		t.Run(name, func(t *testing.T) {
			elfFile := getELF(tc.filePath, t)
			defer elfFile.Close()

			hasDWARF := pfelf.HasDWARFData(elfFile)
			assert.Equal(t, tc.expectedResult, hasDWARF)
		})
	}
}

func TestGetSectionAddress(t *testing.T) {
	elfFile := getELF("testdata/fixed-address", t)
	defer elfFile.Close()

	// The fixed-address test executable has a section named `.coffee_section` at address 0xC0FFEE0
	address, found, err := pfelf.GetSectionAddress(elfFile, ".coffee_section")
	require.NoError(t, err)
	assert.True(t, found, "unable to find .coffee_section")
	assert.Equal(t, uint64(0xC0FFEE0), address)

	// Try to find a section that does not exist
	_, found, err = pfelf.GetSectionAddress(elfFile, ".tea_section")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestGetBuildIDFromNotesFile(t *testing.T) {
	buildID, err := pfelf.GetBuildIDFromNotesFile("testdata/the_notorious_build_id")
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString([]byte("_notorious_build_id_")), buildID)
}

func TestGetKernelVersionBytes(t *testing.T) {
	files := []string{"testdata/kernel-image", "testdata/ubuntu-kernel-image"}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			elfFile := getELF(f, t)
			defer elfFile.Close()

			ver, err := pfelf.GetKernelVersionBytes(elfFile)
			require.NoError(t, err)
			assert.Equal(t, "Linux version 1.2.3\n", string(ver))
		})
	}
}

func TestSymbols(t *testing.T) {
	exePath, err := testsupport.WriteSharedLibrary()
	require.NoError(t, err)
	defer os.Remove(exePath)

	ef, err := elf.Open(exePath)
	require.NoError(t, err)
	defer ef.Close()

	symmap, err := pfelf.GetDynamicSymbols(ef)
	require.NoError(t, err)

	sym, _ := symmap.LookupSymbol("func")
	if assert.NotNil(t, sym) {
		assert.Equal(t, libpf.SymbolValue(0x1000), sym.Address)
	}
	sym, _ = symmap.LookupSymbol("not_existent")
	assert.Nil(t, sym)

	name, offs, ok := symmap.LookupByAddress(0x1002)
	if assert.True(t, ok) {
		assert.Equal(t, libpf.SymbolName("func"), name)
		assert.Equal(t, libpf.Address(2), offs)
	}
}

func testGoBinary(t *testing.T, filename string, isGoExpected bool) {
	ef := getELF(filename, t)
	defer ef.Close()

	isGo, err := pfelf.IsGoBinary(ef)
	require.NoError(t, err)
	assert.Equal(t, isGoExpected, isGo)
}

func TestIsGoBinary(t *testing.T) {
	testGoBinary(t, "testdata/go-binary", true)
	testGoBinary(t, "testdata/without-debug-syms", false)
}

func TestHasCodeSection(t *testing.T) {
	tests := map[string]struct {
		filePath       string
		expectedResult bool
	}{
		"ELF executable - no DWARF":   {withoutDebugSymsPath, true},
		"ELF executable - with DWARF": {withDebugSymsPath, true},
		"Separate debug symbols":      {separateDebugFile, false},
	}

	for testName, testCase := range tests {
		name := testName
		tc := testCase
		t.Run(name, func(t *testing.T) {
			elfFile := getELF(tc.filePath, t)
			defer elfFile.Close()

			hasCode := pfelf.HasCodeSection(elfFile)
			assert.Equal(t, tc.expectedResult, hasCode)
		})
	}
}
