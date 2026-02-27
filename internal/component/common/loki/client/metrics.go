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
	reasonQueueIsFull   = "queue_is_full"
)

var reasons = []string{reasonGeneric, reasonRateLimited, reasonStreamLimited, reasonLineTooLong, reasonQueueIsFull}

type metrics struct {
	sentBytes                    *prometheus.CounterVec
	droppedBytes                 *prometheus.CounterVec
	sentEntries                  *prometheus.CounterVec
	droppedEntries               *prometheus.CounterVec
	requestSize                  *prometheus.HistogramVec
	requestDuration              *prometheus.HistogramVec
	batchRetries                 *prometheus.CounterVec
	entryLatency                 *prometheus.HistogramVec
	countersWithHostTenant       []*prometheus.CounterVec
	countersWithHostTenantReason []*prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics

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

	const (
		KiB = 1024
		MiB = 1024 * KiB
	)

	m.entryLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "loki_write_entry_propagation_latency",
		Help:    "Write latency for for entries",
		Buckets: []float64{0.0002, 0.001, 0.005, 0.02, 0.1, 1.0},
	}, []string{labelHost, labelTenant})
	m.requestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "loki_write_request_size_bytes",
		Help:    "Number of bytes for requests.",
		Buckets: []float64{1 * KiB, 4 * KiB, 16 * KiB, 64 * KiB, 256 * KiB, 512 * KiB, 1 * MiB, 2 * MiB, 4 * MiB, 8 * MiB, 16 * MiB, 20 * MiB},
	}, []string{labelHost, labelTenant})
	m.requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "loki_write_request_duration_seconds",
		Help: "Duration of send requests.",
	}, []string{"status_code", labelHost, labelTenant})
	m.batchRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_write_batch_retries_total",
		Help: "Number of times batches has had to be retried.",
	}, []string{labelHost, labelTenant})

	m.countersWithHostTenant = []*prometheus.CounterVec{
		m.batchRetries, m.sentBytes, m.sentEntries,
	}

	m.countersWithHostTenantReason = []*prometheus.CounterVec{
		m.droppedBytes, m.droppedEntries,
	}

	if reg != nil {
		m.sentBytes = util.MustRegisterOrGet(reg, m.sentBytes).(*prometheus.CounterVec)
		m.droppedBytes = util.MustRegisterOrGet(reg, m.droppedBytes).(*prometheus.CounterVec)
		m.sentEntries = util.MustRegisterOrGet(reg, m.sentEntries).(*prometheus.CounterVec)
		m.droppedEntries = util.MustRegisterOrGet(reg, m.droppedEntries).(*prometheus.CounterVec)
		m.entryLatency = util.MustRegisterOrGet(reg, m.entryLatency).(*prometheus.HistogramVec)
		m.requestSize = util.MustRegisterOrGet(reg, m.requestSize).(*prometheus.HistogramVec)
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
