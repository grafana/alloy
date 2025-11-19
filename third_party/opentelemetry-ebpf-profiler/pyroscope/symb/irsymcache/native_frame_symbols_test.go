package irsymcache

import (
	"testing"

	"github.com/grafana/pyroscope/lidia"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

func TestNativeFrameSymbols(t *testing.T) {
	resolver, err := NewFSCache(TableTableFactory{
		Options: []lidia.Option{lidia.WithLines(), lidia.WithFiles()},
	}, Options{
		SizeEntries: 1024,
		Path:        t.TempDir(),
	})
	require.NoError(t, err)

	reference := testElfRef(testLibcFIle)
	fid := testFileId(1)
	err = resolver.ObserveExecutable(fid, reference)
	require.NoError(t, err)
	res := SourceInfo{}
	addr := libpf.AddressOrLineno(0x9bc7e)

	SymbolizeNativeFrame(resolver, libpf.Intern("testmapping"),
		addr,
		fid,
		func(si SourceInfo) {
			res = si
		})
	require.Equal(t, SourceInfo{
		FunctionName: libpf.Intern("__GI___pthread_cond_timedwait"),
	}, res)
}
