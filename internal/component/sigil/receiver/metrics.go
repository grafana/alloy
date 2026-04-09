package receiver

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	requests     *prometheus.CounterVec
	requestBytes *prometheus.CounterVec
	latency      *prometheus.HistogramVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigil_receiver_requests_total",
			Help: "Total number of generation export requests received.",
		}, []string{"status_code"}),
		requestBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigil_receiver_request_body_bytes_total",
			Help: "Total bytes received in generation export requests.",
		}, []string{"status_code"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "sigil_receiver_request_duration_seconds",
			Help: "Duration of generation export request handling.",
		}, []string{}),
	}

	if reg != nil {
		m.requests = util.MustRegisterOrGet(reg, m.requests).(*prometheus.CounterVec)
		m.requestBytes = util.MustRegisterOrGet(reg, m.requestBytes).(*prometheus.CounterVec)
		m.latency = util.MustRegisterOrGet(reg, m.latency).(*prometheus.HistogramVec)
	}

	return m
}
