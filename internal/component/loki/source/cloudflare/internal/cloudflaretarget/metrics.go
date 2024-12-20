package cloudflaretarget

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds a set of cloudflare metrics.
type Metrics struct {
	reg prometheus.Registerer

	Entries prometheus.Counter
	LastEnd prometheus.Gauge
}

// NewMetrics creates a new set of cloudflare metrics. If reg is non-nil, the
// metrics will be registered.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics
	m.reg = reg

	m.Entries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_cloudflare_target_entries_total",
		Help: "Total number of successful entries sent via the cloudflare target.",
	})
	m.LastEnd = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "loki_source_cloudflare_target_last_requested_end_timestamp",
		Help: "The last cloudflare request end timestamp fetched. This allows to calculate how far the target is behind.",
	})

	if reg != nil {
		m.Entries = util.MustRegisterOrGet(reg, m.Entries).(prometheus.Counter)
		m.LastEnd = util.MustRegisterOrGet(reg, m.LastEnd).(prometheus.Gauge)
	}

	return &m
}
