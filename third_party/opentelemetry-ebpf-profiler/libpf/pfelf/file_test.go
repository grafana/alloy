// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pfelf

import (
	"go/version"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/testsupport"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

func getPFELF(path string, t *testing.T) *File {
	file, err := Open(path)
	assert.NoError(t, err)
	return file
}

func TestGnuHash(t *testing.T) {
	assert.Equal(t, uint32(0x00001505), calcGNUHash(""))
	assert.Equal(t, uint32(0x156b2bb8), calcGNUHash("printf"))
	assert.Equal(t, uint32(0x7c967e3f), calcGNUHash("exit"))
	assert.Equal(t, uint32(0xbac212a0), calcGNUHash("syscall"))
}

func lookupSymbolAddress(ef *File, name libpf.SymbolName) libpf.SymbolValue {
	val, _ := ef.LookupSymbolAddress(name)
	return val
}

func TestPFELFSymbols(t *testing.T) {
	exePath, err := testsupport.WriteSharedLibrary()
	require.NoError(t, err)
	defer os.Remove(exePath)

	ef, err := Open(exePath)
	require.NoError(t, err)
	defer ef.Close()

	// Test GNU hash lookup
	assert.Equal(t, libpf.SymbolValue(0x1000), lookupSymbolAddress(ef, "func"))
	assert.Equal(t, libpf.SymbolValueInvalid, lookupSymbolAddress(ef, "not_existent"))

	// Test SYSV lookup
	ef.gnuHash.addr = 0
	assert.Equal(t, libpf.SymbolValue(0x1000), lookupSymbolAddress(ef, "func"))
	assert.Equal(t, libpf.SymbolValueInvalid, lookupSymbolAddress(ef, "not_existent"))
}

func TestPFELFSections(t *testing.T) {
	elfFile, err := Open("testdata/fixed-address")
	require.NoError(t, err)
	defer elfFile.Close()

	// The fixed-address test executable has a section named `.coffee_section` at address 0xC0FFEE0
	sh := elfFile.Section(".coffee_section")
	if assert.NotNil(t, sh) {
		assert.Equal(t, ".coffee_section", sh.Name)
		assert.Equal(t, uint64(0xC0FFEE0), sh.Addr)

		// Try to find a section that does not exist
		sh = elfFile.Section(".tea_section")
		assert.Nil(t, sh)
	}
}

func testPFELFIsGolang(t *testing.T, filename string, isGoExpected bool) {
	ef := getPFELF(filename, t)
	defer ef.Close()
	assert.Equal(t, isGoExpected, ef.IsGolang())
}

func TestPFELFIsGolang(t *testing.T) {
	testPFELFIsGolang(t, "testdata/go-binary", true)
	testPFELFIsGolang(t, "testdata/without-debug-syms", false)
}

func TestGoVersion(t *testing.T) {
	ef := getPFELF("testdata/go-binary", t)
	defer ef.Close()

	vers, err := ef.GoVersion()
	require.NoError(t, err)
	cmp := version.Compare(vers, "go1.23.6")
	t.Logf("%s %v", vers, cmp)
	assert.GreaterOrEqual(t, cmp, 0)

	testEF := getPFELF("/proc/self/exe", t)
	defer testEF.Close()
	testVersion, err := testEF.GoVersion()
	require.NoError(t, err)
	assert.Equal(t, runtime.Version(), testVersion)
}

func TestGetGoBuildID(t *testing.T) {
	ef := getPFELF("testdata/go-binary", t)
	defer ef.Close()

	buildID, err := ef.GetGoBuildID()
	require.NoError(t, err)
	out, err := exec.Command("go", "tool", "buildid", "testdata/go-binary").Output()
	require.NoError(t, err)
	expectedBuildID := strings.TrimRight(string(out), "\n")
	assert.Equal(t, expectedBuildID, buildID)
}
