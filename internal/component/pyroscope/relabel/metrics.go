package relabel

import (
	"github.com/grafana/alloy/internal/util"

	"github.com/prometheus/client_golang/prometheus"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	profilesProcessed prometheus.Counter
	profilesOutgoing  prometheus.Counter
	profilesDropped   prometheus.Counter
	cacheHits         prometheus.Counter
	cacheMisses       prometheus.Counter
	cacheSize         prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
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
		m.profilesProcessed = util.MustRegisterOrGet(reg, m.profilesProcessed).(prometheus_client.Counter)
		m.profilesOutgoing = util.MustRegisterOrGet(reg, m.profilesOutgoing).(prometheus_client.Counter)
		m.profilesDropped = util.MustRegisterOrGet(reg, m.profilesDropped).(prometheus_client.Counter)
		m.cacheHits = util.MustRegisterOrGet(reg, m.cacheHits).(prometheus_client.Counter)
		m.cacheMisses = util.MustRegisterOrGet(reg, m.cacheMisses).(prometheus_client.Counter)
		m.cacheSize = util.MustRegisterOrGet(reg, m.cacheSize).(prometheus_client.Gauge)
	}

	return &m
}
