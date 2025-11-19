// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golang // import "go.opentelemetry.io/ebpf-profiler/interpreter/go"

import (
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/interpreter"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/successfailurecounter"
)

var (
	// compiler check to make sure the needed interfaces are satisfied
	_ interpreter.Data     = &goData{}
	_ interpreter.Instance = &goInstance{}
)

type goData struct {
	refs atomic.Int32

	pclntab *elfunwindinfo.Gopclntab
}

type goInstance struct {
	interpreter.InstanceStubs

	// Go symbolization metrics
	successCount atomic.Uint64
	failCount    atomic.Uint64

	d *goData
}

func Loader(_ interpreter.EbpfHandler, info *interpreter.LoaderInfo) (
	interpreter.Data, error) {
	ef, err := info.GetELF()
	if err != nil {
		return nil, err
	}
	if !ef.IsGolang() {
		return nil, nil
	}

	pclntab, err := elfunwindinfo.NewGopclntab(ef)
	if pclntab == nil {
		return nil, err
	}

	g := &goData{pclntab: pclntab}
	g.refs.Store(1)
	return g, nil
}

func (g *goData) unref() {
	if g.refs.Add(-1) == 0 {
		_ = g.pclntab.Close()
	}
}

func (g *goData) Attach(_ interpreter.EbpfHandler, _ libpf.PID,
	_ libpf.Address, _ remotememory.RemoteMemory) (interpreter.Instance, error) {
	g.refs.Add(1)
	return &goInstance{d: g}, nil
}

func (g *goData) Unload(_ interpreter.EbpfHandler) {
	g.unref()
}

func (g *goInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	return []metrics.Metric{
		{
			ID:    metrics.IDGoSymbolizationSuccess,
			Value: metrics.MetricValue(g.successCount.Swap(0)),
		},
		{
			ID:    metrics.IDGoSymbolizationFailure,
			Value: metrics.MetricValue(g.failCount.Swap(0)),
		},
	}, nil
}

func (g *goInstance) Detach(_ interpreter.EbpfHandler, _ libpf.PID) error {
	g.d.unref()
	return nil
}

func (g *goInstance) Symbolize(frame *host.Frame, frames *libpf.Frames) error {
	if !frame.Type.IsInterpType(libpf.Native) {
		return interpreter.ErrMismatchInterpreterType
	}
	sfCounter := successfailurecounter.New(&g.successCount, &g.failCount)
	defer sfCounter.DefaultToFailure()

	sourceFile, lineNo, fn := g.d.pclntab.Symbolize(uintptr(frame.Lineno))
	if fn == "" {
		return fmt.Errorf("failed to symbolize 0x%x", frame.Lineno)
	}

	frames.Append(&libpf.Frame{
		Type: libpf.GoFrame,
		//TODO: File: convert the frame.File (host.FileID) to libpf.FileID here
		AddressOrLineno: frame.Lineno,
		FunctionName:    libpf.Intern(fn),
		SourceFile:      libpf.Intern(sourceFile),
		SourceLine:      libpf.SourceLineno(lineNo),
	})
	sfCounter.ReportSuccess()
	return nil
}

func (g *goInstance) ReleaseResources() error {
	return g.d.pclntab.SetDontNeed()
}
