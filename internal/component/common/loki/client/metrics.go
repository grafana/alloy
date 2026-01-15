package client

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	labelHost   = "host"
	labelTenant = "tenant"
	labelReason = "reason"

	reasonGeneric       = "ingester_error"
	reasonRateLimited   = "rate_limited"
	reasonStreamLimited = "stream_limited"
	reasonLineTooLong   = "line_too_long"
)

var reasons = []string{reasonGeneric, reasonRateLimited, reasonStreamLimited, reasonLineTooLong}

type metrics struct {
	encodedBytes                 *prometheus.CounterVec
	sentBytes                    *prometheus.CounterVec
	droppedBytes                 *prometheus.CounterVec
	sentEntries                  *prometheus.CounterVec
	droppedEntries               *prometheus.CounterVec
	mutatedEntries               *prometheus.CounterVec
	mutatedBytes                 *prometheus.CounterVec
	requestDuration              *prometheus.HistogramVec
	batchRetries                 *prometheus.CounterVec
	countersWithHostTenant       []*prometheus.CounterVec
	countersWithHostTenantReason []*prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics

	m.encodedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_encoded_bytes_total",
		Help: "Number of bytes encoded and ready to send.",
	}, []string{labelHost, labelTenant})
	m.sentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_sent_bytes_total",
		Help: "Number of bytes sent.",
	}, []string{labelHost, labelTenant})
	m.droppedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_dropped_bytes_total",
		Help: "Number of bytes dropped because failed to be sent to the ingester after all retries.",
	}, []string{labelHost, labelTenant, labelReason})
	m.sentEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_sent_entries_total",
		Help: "Number of log entries sent to the ingester.",
	}, []string{labelHost, labelTenant})
	m.droppedEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_dropped_entries_total",
		Help: "Number of log entries dropped because failed to be sent to the ingester after all retries.",
	}, []string{labelHost, labelTenant, labelReason})
	m.mutatedEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_mutated_entries_total",
		Help: "The total number of log entries that have been mutated.",
	}, []string{labelHost, labelTenant, labelReason})
	m.mutatedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_mutated_bytes_total",
		Help: "The total number of bytes that have been mutated.",
	}, []string{labelHost, labelTenant, labelReason})
	m.requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "loki_write_request_duration_seconds",
		Help: "Duration of send requests.",
	}, []string{"status_code", labelHost, labelTenant})
	m.batchRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_batch_retries_total",
		Help: "Number of times batches has had to be retried.",
	}, []string{labelHost, labelTenant})

	m.countersWithHostTenant = []*prometheus.CounterVec{
		m.batchRetries, m.encodedBytes, m.sentBytes, m.sentEntries,
	}

	m.countersWithHostTenantReason = []*prometheus.CounterVec{
		m.droppedBytes, m.droppedEntries, m.mutatedEntries, m.mutatedBytes,
	}

	if reg != nil {
		m.encodedBytes = util.MustRegisterOrGet(reg, m.encodedBytes).(*prometheus.CounterVec)
		m.sentBytes = util.MustRegisterOrGet(reg, m.sentBytes).(*prometheus.CounterVec)
		m.droppedBytes = util.MustRegisterOrGet(reg, m.droppedBytes).(*prometheus.CounterVec)
		m.sentEntries = util.MustRegisterOrGet(reg, m.sentEntries).(*prometheus.CounterVec)
		m.droppedEntries = util.MustRegisterOrGet(reg, m.droppedEntries).(*prometheus.CounterVec)
		m.mutatedEntries = util.MustRegisterOrGet(reg, m.mutatedEntries).(*prometheus.CounterVec)
		m.mutatedBytes = util.MustRegisterOrGet(reg, m.mutatedBytes).(*prometheus.CounterVec)
		m.requestDuration = util.MustRegisterOrGet(reg, m.requestDuration).(*prometheus.HistogramVec)
		m.batchRetries = util.MustRegisterOrGet(reg, m.batchRetries).(*prometheus.CounterVec)
	}

	return &m
}

type walEndpointMetrics struct {
	lastReadTimestamp *prometheus.GaugeVec
}

func newWALEndpointMetrics(reg prometheus.Registerer) *walEndpointMetrics {
	m := &walEndpointMetrics{
		lastReadTimestamp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "loki_write",
				Name:      "last_read_timestamp",
				Help:      "Latest timestamp read from the WAL",
			},
			[]string{"id"},
		),
	}

	if reg != nil {
		m.lastReadTimestamp = util.MustRegisterOrGet(reg, m.lastReadTimestamp).(*prometheus.GaugeVec)
	}

	return m
}

func (m *walEndpointMetrics) CurryWithId(id string) *walEndpointMetrics {
	return &walEndpointMetrics{
		lastReadTimestamp: m.lastReadTimestamp.MustCurryWith(map[string]string{
			"id": id,
		}),
	}
}
