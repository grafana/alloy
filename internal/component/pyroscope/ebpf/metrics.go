//go:build linux && (arm64 || amd64)

// the build tag is to avoid unnecessary compilation of symtab

package ebpf

import (
	"github.com/grafana/alloy/internal/util"
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
	pprofsDroppedTotal            prometheus.Counter
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
		pprofsDroppedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_ebpf_pprofs_dropped_total",
			Help: "Total number of pprof profiles dropped by the ebpf component",
		}),
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
		m.targetsActive = util.MustRegisterOrGet(reg, m.targetsActive).(prometheus.Gauge)
		m.profilingSessionsTotal = util.MustRegisterOrGet(reg, m.profilingSessionsTotal).(prometheus.Counter)
		m.profilingSessionsFailingTotal = util.MustRegisterOrGet(reg, m.profilingSessionsFailingTotal).(prometheus.Counter)
		m.pprofsTotal = util.MustRegisterOrGet(reg, m.pprofsTotal).(*prometheus.CounterVec)
		m.pprofBytesTotal = util.MustRegisterOrGet(reg, m.pprofBytesTotal).(*prometheus.CounterVec)
		m.pprofSamplesTotal = util.MustRegisterOrGet(reg, m.pprofSamplesTotal).(*prometheus.CounterVec)
		m.pprofsDroppedTotal = util.MustRegisterOrGet(reg, m.pprofsDroppedTotal).(prometheus.Counter)
	}

	return m
}
