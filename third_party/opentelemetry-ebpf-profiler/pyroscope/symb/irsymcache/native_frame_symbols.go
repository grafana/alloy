package irsymcache // import "go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"

import (
	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
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
		si  SourceInfo
		err error
	)
	if mappingName != process.VdsoPathName {
		si, err = resolver.ResolveAddress(fileID, uint64(addr))
		if err != nil {
			logrus.Debugf("Failed to symbolize %v %x %v", fileID.StringNoQuotes(), addr, err)
		}
	}
	symbolize(si)
}
