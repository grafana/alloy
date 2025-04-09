package write

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	sentBytes       *prometheus.CounterVec
	droppedBytes    *prometheus.CounterVec
	sentProfiles    *prometheus.CounterVec
	droppedProfiles *prometheus.CounterVec
	retries         *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		sentBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_write_sent_bytes_total",
			Help: "Total number of compressed bytes sent to Pyroscope.",
		}, []string{"endpoint"}),
		droppedBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_write_dropped_bytes_total",
			Help: "Total number of compressed bytes dropped by Pyroscope.",
		}, []string{"endpoint"}),
		sentProfiles: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_write_sent_profiles_total",
			Help: "Total number of profiles sent to Pyroscope.",
		}, []string{"endpoint"}),
		droppedProfiles: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_write_dropped_profiles_total",
			Help: "Total number of profiles dropped by Pyroscope.",
		}, []string{"endpoint"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_write_retries_total",
			Help: "Total number of retries to Pyroscope.",
		}, []string{"endpoint"}),
	}

	if reg != nil {
		m.sentBytes = util.MustRegisterOrGet(reg, m.sentBytes).(*prometheus.CounterVec)
		m.droppedBytes = util.MustRegisterOrGet(reg, m.droppedBytes).(*prometheus.CounterVec)
		m.sentProfiles = util.MustRegisterOrGet(reg, m.sentProfiles).(*prometheus.CounterVec)
		m.droppedProfiles = util.MustRegisterOrGet(reg, m.droppedProfiles).(*prometheus.CounterVec)
		m.retries = util.MustRegisterOrGet(reg, m.retries).(*prometheus.CounterVec)
	}

	return m
}
