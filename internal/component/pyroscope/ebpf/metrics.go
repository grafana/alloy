//go:build linux

// the build tag is to avoid unnecessary compilation of symtab

package ebpf

import (
	ebpfmetrics "github.com/grafana/pyroscope/ebpf/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	targetsActive                 prometheus.Gauge
	profilingSessionsTotal        prometheus.Counter
	profilingSessionsFailingTotal prometheus.Counter
	pprofsTotal                   *prometheus.CounterVec
	pprofBytesTotal               *prometheus.CounterVec
	pprofSamplesTotal             *prometheus.CounterVec
	ebpfMetrics                   *ebpfmetrics.Metrics
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		targetsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pyroscope_ebpf_active_targets",
			Help: "Current number of active targets being tracked by the ebpf component",
		}),
		profilingSessionsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_ebpf_profiling_sessions_total",
			Help: "Total number of profiling sessions started by the ebpf component",
		}),
		profilingSessionsFailingTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_ebpf_profiling_sessions_failing_total",
			Help: "Total number of profiling sessions failed to complete by the ebpf component",
		}),
		pprofsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_ebpf_pprofs_total",
			Help: "Total number of pprof profiles collected by the ebpf component",
		}, []string{"service_name"}),
		pprofBytesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_ebpf_pprof_bytes_total",
			Help: "Total number of pprof profiles collected by the ebpf component",
		}, []string{"service_name"}),
		pprofSamplesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_ebpf_pprof_samples_total",
			Help: "Total number of pprof profiles collected by the ebpf component",
		}, []string{"service_name"}),
		ebpfMetrics: ebpfmetrics.New(reg),
	}

	if reg != nil {
		reg.MustRegister(
			m.targetsActive,
			m.profilingSessionsTotal,
			m.profilingSessionsFailingTotal,
			m.pprofsTotal,
			m.pprofBytesTotal,
			m.pprofSamplesTotal,
		)
	}

	return m
}
