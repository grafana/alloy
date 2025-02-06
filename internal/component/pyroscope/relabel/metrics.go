package relabel

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	profilesProcessed prometheus.Counter
	profilesOutgoing  prometheus.Counter
	profilesDropped   prometheus.Counter
	cacheHits         prometheus.Counter
	cacheMisses       prometheus.Counter
	cacheSize         prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		profilesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_relabel_profiles_processed",
			Help: "Total number of profiles processed",
		}),
		profilesOutgoing: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_relabel_profiles_written",
			Help: "Total number of profiles forwarded",
		}),
		profilesDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_relabel_profiles_dropped",
			Help: "Total number of profiles dropped by relabeling rules",
		}),
		cacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_relabel_cache_hits",
			Help: "Total number of cache hits",
		}),
		cacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_relabel_cache_misses",
			Help: "Total number of cache misses",
		}),
		cacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pyroscope_relabel_cache_size",
			Help: "Total size of relabel cache",
		}),
	}

	reg.MustRegister(
		m.profilesProcessed,
		m.profilesOutgoing,
		m.profilesDropped,
		m.cacheHits,
		m.cacheMisses,
		m.cacheSize,
	)

	return m
}
