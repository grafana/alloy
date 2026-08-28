//go:build unix

package irsymcache

import (
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

type NativeSymbolResolver interface {
	ResolveAddress(file libpf.FileID, addr uint64) (SourceInfo, error)
	Cleanup()
}
type SourceInfo struct {
	LineNumber   libpf.SourceLineno
	FunctionName libpf.String
	FilePath     libpf.String
}

func SymbolizeNativeFrame(
	resolver NativeSymbolResolver,

	mappingName libpf.String,
	addr libpf.AddressOrLineno,
	fileID libpf.FileID,
	symbolize func(si SourceInfo),
) {

	var (
		si SourceInfo
	)
	if mappingName != vdsoPathName {
		si, _ = resolver.ResolveAddress(fileID, uint64(addr))
	}
	symbolize(si)
}
