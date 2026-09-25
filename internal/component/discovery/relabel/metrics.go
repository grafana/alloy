package relabel

import (
	prometheus_client "github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	cacheHits   prometheus_client.Counter
	cacheMisses prometheus_client.Counter
	cacheSize   prometheus_client.Gauge
}

func newMetrics(reg prometheus_client.Registerer) *metrics {
	m := metrics{
		cacheHits: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "alloy_discovery_relabel_cache_hits_total",
			Help: "Total number of targets whose relabeling result was reused from the cache",
		}),
		cacheMisses: prometheus_client.NewCounter(prometheus_client.CounterOpts{
			Name: "alloy_discovery_relabel_cache_misses_total",
			Help: "Total number of targets that had to be relabeled",
		}),
		cacheSize: prometheus_client.NewGauge(prometheus_client.GaugeOpts{
			Name: "alloy_discovery_relabel_cache_size",
			Help: "Number of targets currently held in the relabeling cache",
		}),
	}

	if reg != nil {
		m.cacheHits = util.MustRegisterOrGet(reg, m.cacheHits).(prometheus_client.Counter)
		m.cacheMisses = util.MustRegisterOrGet(reg, m.cacheMisses).(prometheus_client.Counter)
		m.cacheSize = util.MustRegisterOrGet(reg, m.cacheSize).(prometheus_client.Gauge)
	}

	return &m
}
