package producer

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	requests        *prometheus.CounterVec
	producedBytes   *prometheus.CounterVec
	produceDuration prometheus.Histogram
	rejected        *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_tenant_producer_requests_total",
			Help: "Total number of ingest requests by signal, format and response code.",
		}, []string{"signal", "format", "code"}),
		producedBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_tenant_producer_produced_bytes_total",
			Help: "Total number of request body bytes produced to Kafka.",
		}, []string{"signal"}),
		produceDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "kafka_tenant_producer_produce_duration_seconds",
			Help:    "Time until a produced record was acknowledged or failed.",
			Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}),
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_tenant_producer_rejected_total",
			Help: "Total number of requests rejected before producing.",
		}, []string{"reason"}),
	}
	m.requests = util.MustRegisterOrGet(reg, m.requests).(*prometheus.CounterVec)
	m.producedBytes = util.MustRegisterOrGet(reg, m.producedBytes).(*prometheus.CounterVec)
	m.produceDuration = util.MustRegisterOrGet(reg, m.produceDuration).(prometheus.Histogram)
	m.rejected = util.MustRegisterOrGet(reg, m.rejected).(*prometheus.CounterVec)
	return m
}
