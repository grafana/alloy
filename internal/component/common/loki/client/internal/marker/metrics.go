package marker

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	lastMarkedSegment *prometheus.GaugeVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
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

// CurryWithId returns a curried version of MarkerMetrics, with the id label pre-filled. This is a helper that avoids
// having to move the id around where it's unnecessary, and won't change inside the consumer of the metrics.
func (m *Metrics) CurryWithId(id string) *Metrics {
	return &Metrics{
		lastMarkedSegment: m.lastMarkedSegment.MustCurryWith(map[string]string{
			"id": id,
		}),
	}
}
