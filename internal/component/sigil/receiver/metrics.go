package receiver

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	partialFailures prometheus.Counter
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		partialFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sigil_receive_fanout_partial_failures_total",
			Help: "Number of times fan-out to downstream receivers had at least one failure but at least one success.",
		}),
	}

	if reg != nil {
		m.partialFailures = util.MustRegisterOrGet(reg, m.partialFailures).(prometheus.Counter)
	}

	return m
}
