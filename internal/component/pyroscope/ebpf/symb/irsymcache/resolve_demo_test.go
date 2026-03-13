//go:build unix

package irsymcache

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

const testDemoFile = "../testdata/demo"

// TODO: We need a fallback for resolving a function with zero STT_FUNC type
// (e.g. start_fiber at 0x401774 in the demo binary has size 0 and is not resolved by lidia).
func TestResolveDemoAddress(t *testing.T) {
	resolver, err := NewFSCache(log.NewNopLogger(), tf, Options{
		Path:        t.TempDir(),
		SizeEntries: 1024,
	})
	require.NoError(t, err)
	defer resolver.Close()

	fid := testFileId(1)
	md := testElfRef(testDemoFile)
	err = resolver.ObserveExecutable(fid, md)
	require.NoError(t, err)

	si, err := resolver.ResolveAddress(fid, 0x401774)
	require.NoError(t, err)

	t.Logf("FunctionName: %s", si.FunctionName)
	t.Logf("FilePath:     %s", si.FilePath)
	t.Logf("LineNumber:   %d", si.LineNumber)
}
