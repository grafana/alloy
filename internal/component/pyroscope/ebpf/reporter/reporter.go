package reporter

import (
	"errors"
	"github.com/elastic/go-freelru"
	"github.com/go-kit/log"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	pyrosd "go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/controller"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/internalshim/helpers"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/samples"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	samples2 "go.opentelemetry.io/ebpf-profiler/reporter/samples"
	"go.opentelemetry.io/ebpf-profiler/times"
	"google.golang.org/grpc"

	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/auth"
)

func New(
	log log.Logger,
	cgroups *freelru.SyncedLRU[libpf.PID, string],
	cfg *controller.Config,
	sd pyrosd.TargetProducer,
	nfs samples2.NativeSymbolResolver,
	consumer PPROFConsumer,
) (reporter.Reporter, error) {
	intervals := times.New(cfg.MonitorInterval,
		cfg.ReporterInterval, cfg.ProbabilisticInterval)
	kernelVersion, err := helpers.GetKernelVersion()
	if err != nil {
		return nil, err
	}

	otelReporter := false
	if cfg.PyroscopeReporterType == "otel" || cfg.PyroscopeReporterType == "otlp" {
		otelReporter = true
	}
	if !otelReporter {
		return NewPPROF(log, cgroups, &Config{
			ExtraNativeSymbolResolver: nfs,
			CGroupCacheElements:       1024,
			ReportInterval:            cfg.ReporterInterval,
			SamplesPerSecond:          int64(cfg.SamplesPerSecond),
			ExecutablesCacheElements:  16384,
			FramesCacheElements:       65536,
			Consumer:                  consumer,
		}, sd)
	}
	sap := samples.NewAttributesProviderFromDiscovery(sd)

	var dialOption []grpc.DialOption
	if cfg.PyroscopeUsername != "" && cfg.PyroscopePasswordFile != "" {
		opt, err := auth.NewBasicAuth(cfg.PyroscopeUsername, cfg.PyroscopePasswordFile)
		if err != nil {
			return nil, err
		}
		dialOption = append(dialOption, grpc.WithDefaultCallOptions(opt))
	}

	hostname, sourceIP, err := helpers.GetHostnameAndSourceIP(cfg.CollAgentAddr)
	if err != nil {
		return nil, err
	}
	cfg.HostName, cfg.IPAddress = hostname, sourceIP
	if cfg.CollAgentAddr == "" {
		return nil, errors.New("missing otlp collector address")
	}
	reporterConfig := &reporter.Config{
		CollAgentAddr:            cfg.CollAgentAddr,
		DisableTLS:               cfg.DisableTLS,
		MaxRPCMsgSize:            32 << 20, // 32 MiB
		MaxGRPCRetries:           5,
		GRPCOperationTimeout:     intervals.GRPCOperationTimeout(),
		GRPCStartupBackoffTime:   intervals.GRPCStartupBackoffTime(),
		GRPCConnectionTimeout:    intervals.GRPCConnectionTimeout(),
		ReportInterval:           intervals.ReportInterval(),
		ExecutablesCacheElements: 16384,
		// Next step: Calculate FramesCacheElements from numCores and samplingRate.
		FramesCacheElements: 65536,
		CGroupCacheElements: 1024,
		SamplesPerSecond:    cfg.SamplesPerSecond,
		KernelVersion:       kernelVersion,
		HostName:            hostname,
		IPAddress:           sourceIP,

		GRPCDialOptions:           dialOption,
		ExtraNativeSymbolResolver: nfs,
		ExtraSampleAttrProd:       sap,
	}
	return reporter.NewOTLP(reporterConfig, cgroups)
}
