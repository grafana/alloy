//go:build linux && (arm64 || amd64)

package pyroscope

import (
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
)

type DebugInfoData struct {
	FileID   libpf.FileID
	FileName string
	BuildID  string
	Open     func() (process.ReadAtCloser, error)
}
