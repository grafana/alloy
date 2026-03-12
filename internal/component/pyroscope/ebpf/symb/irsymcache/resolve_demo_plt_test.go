//go:build unix

package irsymcache

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestResolveDemoPLT(t *testing.T) {
	resolver, err := NewFSCache(log.NewNopLogger(), tf, Options{
		Path:        t.TempDir(),
		SizeEntries: 1024,
	})
	require.NoError(t, err)
	defer resolver.Close()

	fid := testFileId(2)
	md := testElfRef(testDemoFile)
	err = resolver.ObserveExecutable(fid, md)
	require.NoError(t, err)

	// sin@plt is at 0x400380
	si, err := resolver.ResolveAddress(fid, 0x400380)
	require.NoError(t, err)
	t.Logf("sin@plt 0x400380: FunctionName=%s FilePath=%s LineNumber=%d", si.FunctionName, si.FilePath, si.LineNumber)

	// cos@plt is at 0x400450
	si, err = resolver.ResolveAddress(fid, 0x400450)
	require.NoError(t, err)
	t.Logf("cos@plt 0x400450: FunctionName=%s FilePath=%s LineNumber=%d", si.FunctionName, si.FilePath, si.LineNumber)
}
