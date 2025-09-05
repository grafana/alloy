package relabel

import (
	"github.com/grafana/alloy/internal/util/slim"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	profilesProcessed prometheus_client.Counter
	profilesOutgoing  prometheus_client.Counter
	profilesDropped   prometheus_client.Counter
	cacheHits         prometheus_client.Counter
	cacheMisses       prometheus_client.Counter
	cacheSize         prometheus_client.Gauge
}

func newMetrics(reg prometheus_client.Registerer) *metrics {
	m := metrics{
		profilesProcessed: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_relabel_profiles_processed",
			Help: "Total number of profiles processed",
		}),
		profilesOutgoing: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_relabel_profiles_written",
			Help: "Total number of profiles forwarded",
		}),
		profilesDropped: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_relabel_profiles_dropped",
			Help: "Total number of profiles dropped by relabeling rules",
		}),
		cacheHits: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_relabel_cache_hits",
			Help: "Total number of cache hits",
		}),
		cacheMisses: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_relabel_cache_misses",
			Help: "Total number of cache misses",
		}),
		cacheSize: prometheus_client.NewGauge(prometheus_client.GaugeOpts{
			Name: "pyroscope_relabel_cache_size",
			Help: "Total size of relabel cache",
		}),
	}

	if reg != nil {
		m.profilesProcessed = slim.MustRegisterOrGet(reg, m.profilesProcessed).(prometheus_client.Counter)
		m.profilesOutgoing = slim.MustRegisterOrGet(reg, m.profilesOutgoing).(prometheus_client.Counter)
		m.profilesDropped = slim.MustRegisterOrGet(reg, m.profilesDropped).(prometheus_client.Counter)
		m.cacheHits = slim.MustRegisterOrGet(reg, m.cacheHits).(prometheus_client.Counter)
		m.cacheMisses = slim.MustRegisterOrGet(reg, m.cacheMisses).(prometheus_client.Counter)
		m.cacheSize = slim.MustRegisterOrGet(reg, m.cacheSize).(prometheus_client.Gauge)
	}

	return &m
}
