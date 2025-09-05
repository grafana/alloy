package client

import (
	"github.com/grafana/alloy/internal/util/slim"
	"github.com/prometheus/client_golang/prometheus"
)

type QueueClientMetrics struct {
	lastReadTimestamp *prometheus.GaugeVec
}

func NewQueueClientMetrics(reg prometheus.Registerer) *QueueClientMetrics {
	m := &QueueClientMetrics{
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
		m.lastReadTimestamp = slim.MustRegisterOrGet(reg, m.lastReadTimestamp).(*prometheus.GaugeVec)
	}

	return m
}

func (m *QueueClientMetrics) CurryWithId(id string) *QueueClientMetrics {
	return &QueueClientMetrics{
		lastReadTimestamp: m.lastReadTimestamp.MustCurryWith(map[string]string{
			"id": id,
		}),
	}
}
