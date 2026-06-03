package write

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

const endpointLabel = "endpoint"

type metrics struct {
	sentBytes    *prometheus.CounterVec
	droppedBytes *prometheus.CounterVec
	requests     *prometheus.CounterVec
	retries      *prometheus.CounterVec
	latency      *prometheus.HistogramVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		sentBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigil_write_sent_bytes_total",
			Help: "Total number of bytes sent to Sigil.",
		}, []string{endpointLabel}),
		droppedBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigil_write_dropped_bytes_total",
			Help: "Total number of bytes dropped by Sigil write.",
		}, []string{endpointLabel}),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigil_write_requests_total",
			Help: "Total number of requests sent to Sigil.",
		}, []string{endpointLabel, "status_code"}),
		retries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigil_write_retries_total",
			Help: "Total number of retries to Sigil.",
		}, []string{endpointLabel}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "sigil_write_latency",
			Help: "Write latency for sending generations to Sigil.",
		}, []string{endpointLabel}),
	}

	if reg != nil {
		m.sentBytes = util.MustRegisterOrGet(reg, m.sentBytes).(*prometheus.CounterVec)
		m.droppedBytes = util.MustRegisterOrGet(reg, m.droppedBytes).(*prometheus.CounterVec)
		m.requests = util.MustRegisterOrGet(reg, m.requests).(*prometheus.CounterVec)
		m.retries = util.MustRegisterOrGet(reg, m.retries).(*prometheus.CounterVec)
		m.latency = util.MustRegisterOrGet(reg, m.latency).(*prometheus.HistogramVec)
	}

	return m
}
