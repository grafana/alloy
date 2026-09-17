package savepoint

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type Metrics struct {
	lastSavedSegment *prometheus.GaugeVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		lastSavedSegment: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "loki_write",
				Subsystem: "wal_savepoint",
				Name:      "last_saved_segment",
				Help:      "Last saved WAL segment.",
			},
			[]string{"id"},
		),
	}
	if reg != nil {
		m.lastSavedSegment = util.MustRegisterOrGet(reg, m.lastSavedSegment).(*prometheus.GaugeVec)
	}
	return m
}

// curryWithId returns a curried version of Metrics, with the id label pre-filled. This is a helper that avoids
// having to move the id around where it's unnecessary, and won't change inside the consumer of the metrics.
func (m *Metrics) curryWithId(id string) *Metrics {
	return &Metrics{
		lastSavedSegment: m.lastSavedSegment.MustCurryWith(map[string]string{
			"id": id,
		}),
	}
}
