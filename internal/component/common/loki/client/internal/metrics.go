package internal

import (
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type MarkerMetrics struct {
	lastMarkedSegment *prometheus.GaugeVec
}

func NewMarkerMetrics(reg prometheus.Registerer) *MarkerMetrics {
	m := &MarkerMetrics{
		lastMarkedSegment: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "loki_write",
				Subsystem: "wal_marker",
				Name:      "last_marked_segment",
				Help:      "Last marked WAL segment.",
			},
			[]string{"id"},
		),
	}
	if reg != nil {
		m.lastMarkedSegment = util.MustRegisterOrGet(reg, m.lastMarkedSegment).(*prometheus.GaugeVec)
	}
	return m
}

// WithCurriedId returns a curried version of MarkerMetrics, with the id label pre-filled. This is a helper that avoids
// having to move the id around where it's unnecessary, and won't change inside the consumer of the metrics.
func (m *MarkerMetrics) WithCurriedId(id string) *MarkerMetrics {
	return &MarkerMetrics{
		lastMarkedSegment: m.lastMarkedSegment.MustCurryWith(map[string]string{
			"id": id,
		}),
	}
}
