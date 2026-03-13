//go:build unix

package irsymcache

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// TODO: symbols like __GI___clone3 are confusing in profiles, find a way to use a proper/nicer name.
func TestResolveClone3(t *testing.T) {
	resolver, err := NewFSCache(log.NewNopLogger(), tf, Options{
		Path:        t.TempDir(),
		SizeEntries: 1024,
	})
	require.NoError(t, err)
	defer resolver.Close()

	fid := testFileId(3)
	md := testElfRef(testLibcFIle)
	err = resolver.ObserveExecutable(fid, md)
	require.NoError(t, err)

	// 0x129c10 has both __clone3 and __GI___clone3 symbols
	// We expect the resolver to return "__clone3", not "__GI___clone3"
	si, err := resolver.ResolveAddress(fid, 0x129c10)
	require.NoError(t, err)
	require.Equal(t, libpf.Intern("__clone3"), si.FunctionName)
}
