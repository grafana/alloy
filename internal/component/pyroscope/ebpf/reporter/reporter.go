//go:build linux && (arm64 || amd64) && pyroscope_ebpf

package reporter

import (
	"time"

	"github.com/elastic/go-freelru"
	"github.com/go-kit/log"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	pyrosd "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	samples2 "go.opentelemetry.io/ebpf-profiler/reporter/samples"
)

func New(
	log log.Logger,
	cfg *controller.Config,
	sd pyrosd.TargetProducer,
	nfs samples2.NativeSymbolResolver,
	consumer PPROFConsumer,
) (reporter.Reporter, error) {

	return NewPPROF(log, &Config{
		ExtraNativeSymbolResolver: nfs,
		CGroupCacheElements:       1024,
		ReportInterval:            cfg.ReporterInterval,
		SamplesPerSecond:          int64(cfg.SamplesPerSecond),
		ExecutablesCacheElements:  16384,
		FramesCacheElements:       65536,
		Consumer:                  consumer,
	}, sd)

}

func NewContainerIDCache(size uint32) (freelru.Cache[libpf.PID, string], error) {
	var cgroups freelru.Cache[libpf.PID, string]
	var err error
	h := func(pid libpf.PID) uint32 { return uint32(pid) }
	cgroups, err = freelru.New[libpf.PID, string](size, h)
	if err != nil {
		return nil, err
	}
	cgroups.SetLifetime(5 * time.Minute)
	return cgroups, nil
}
