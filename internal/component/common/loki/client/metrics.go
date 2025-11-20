package client

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	HostLabel   = "host"
	TenantLabel = "tenant"
	ReasonLabel = "reason"

	ReasonGeneric       = "ingester_error"
	ReasonRateLimited   = "rate_limited"
	ReasonStreamLimited = "stream_limited"
	ReasonLineTooLong   = "line_too_long"
)

var Reasons = []string{ReasonGeneric, ReasonRateLimited, ReasonStreamLimited, ReasonLineTooLong}

type Metrics struct {
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

func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics

	m.encodedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_encoded_bytes_total",
		Help: "Number of bytes encoded and ready to send.",
	}, []string{HostLabel, TenantLabel})
	m.sentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_sent_bytes_total",
		Help: "Number of bytes sent.",
	}, []string{HostLabel, TenantLabel})
	m.droppedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_dropped_bytes_total",
		Help: "Number of bytes dropped because failed to be sent to the ingester after all retries.",
	}, []string{HostLabel, TenantLabel, ReasonLabel})
	m.sentEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_sent_entries_total",
		Help: "Number of log entries sent to the ingester.",
	}, []string{HostLabel, TenantLabel})
	m.droppedEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_dropped_entries_total",
		Help: "Number of log entries dropped because failed to be sent to the ingester after all retries.",
	}, []string{HostLabel, TenantLabel, ReasonLabel})
	m.mutatedEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_mutated_entries_total",
		Help: "The total number of log entries that have been mutated.",
	}, []string{HostLabel, TenantLabel, ReasonLabel})
	m.mutatedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_mutated_bytes_total",
		Help: "The total number of bytes that have been mutated.",
	}, []string{HostLabel, TenantLabel, ReasonLabel})
	m.requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "loki_write_request_duration_seconds",
		Help: "Duration of send requests.",
	}, []string{"status_code", HostLabel, TenantLabel})
	m.batchRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_batch_retries_total",
		Help: "Number of times batches has had to be retried.",
	}, []string{HostLabel, TenantLabel})

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

type WALEndpointMetrics struct {
	lastReadTimestamp *prometheus.GaugeVec
}

func NewWALEndpointMetrics(reg prometheus.Registerer) *WALEndpointMetrics {
	m := &WALEndpointMetrics{
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

func (m *WALEndpointMetrics) CurryWithId(id string) *WALEndpointMetrics {
	return &WALEndpointMetrics{
		lastReadTimestamp: m.lastReadTimestamp.MustCurryWith(map[string]string{
			"id": id,
		}),
	}
}
