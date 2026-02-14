package relabel

import (
	"github.com/grafana/alloy/internal/util"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	profilesProcessed  prometheus_client.Counter
	profilesOutgoing   prometheus_client.Counter
	profilesDropped    prometheus_client.Counter
	samplesProcessed   prometheus_client.Counter
	samplesOutgoing    prometheus_client.Counter
	samplesDropped     prometheus_client.Counter
	pprofParseFailures prometheus_client.Counter
	pprofWriteFailures prometheus_client.Counter
	cacheHits          prometheus_client.Counter
	cacheMisses        prometheus_client.Counter
	cacheSize          prometheus_client.Gauge
}

func newMetrics(reg prometheus_client.Registerer) *metrics {
	m := metrics{
		profilesProcessed: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_profiles_processed",
			Help: "Total number of profiles processed",
		}),
		profilesOutgoing: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_profiles_written",
			Help: "Total number of profiles forwarded",
		}),
		profilesDropped: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_profiles_dropped",
			Help: "Total number of profiles dropped after relabeling sample labels",
		}),
		samplesProcessed: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_samples_processed",
			Help: "Total number of pprof samples processed",
		}),
		samplesOutgoing: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_samples_written",
			Help: "Total number of pprof samples forwarded",
		}),
		samplesDropped: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_samples_dropped",
			Help: "Total number of pprof samples dropped by relabeling rules",
		}),
		pprofParseFailures: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_pprof_parse_failures",
			Help: "Total number of profiles that could not be parsed as pprof and were forwarded unchanged",
		}),
		pprofWriteFailures: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_pprof_write_failures",
			Help: "Total number of profiles that failed pprof serialization and were forwarded unchanged",
		}),
		cacheHits: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_cache_hits",
			Help: "Total number of cache hits",
		}),
		cacheMisses: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "pyroscope_profile_relabel_cache_misses",
			Help: "Total number of cache misses",
		}),
		cacheSize: prometheus_client.NewGauge(prometheus_client.GaugeOpts{
			Name: "pyroscope_profile_relabel_cache_size",
			Help: "Total size of relabel cache",
		}),
	}

	if reg != nil {
		m.profilesProcessed = util.MustRegisterOrGet(reg, m.profilesProcessed).(prometheus_client.Counter)
		m.profilesOutgoing = util.MustRegisterOrGet(reg, m.profilesOutgoing).(prometheus_client.Counter)
		m.profilesDropped = util.MustRegisterOrGet(reg, m.profilesDropped).(prometheus_client.Counter)
		m.samplesProcessed = util.MustRegisterOrGet(reg, m.samplesProcessed).(prometheus_client.Counter)
		m.samplesOutgoing = util.MustRegisterOrGet(reg, m.samplesOutgoing).(prometheus_client.Counter)
		m.samplesDropped = util.MustRegisterOrGet(reg, m.samplesDropped).(prometheus_client.Counter)
		m.pprofParseFailures = util.MustRegisterOrGet(reg, m.pprofParseFailures).(prometheus_client.Counter)
		m.pprofWriteFailures = util.MustRegisterOrGet(reg, m.pprofWriteFailures).(prometheus_client.Counter)
		m.cacheHits = util.MustRegisterOrGet(reg, m.cacheHits).(prometheus_client.Counter)
		m.cacheMisses = util.MustRegisterOrGet(reg, m.cacheMisses).(prometheus_client.Counter)
		m.cacheSize = util.MustRegisterOrGet(reg, m.cacheSize).(prometheus_client.Gauge)
	}

	return &m
}
