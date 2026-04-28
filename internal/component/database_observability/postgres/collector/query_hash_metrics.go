package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// QueryHashMetricsCollector exposes the QueryHashRegistry as a Prometheus
// "info" gauge metric. The metric is intended to be join-merged into existing
// pg_stat_statements series via PromQL:
//
//	pg_stat_statements_calls_total
//	  * on(queryid, datname) group_left(query_fingerprint)
//	    database_observability_query_hash_info
//
// We deliberately keep the fingerprint *off* the existing pg_stat_statements
// series labels so we don't bump cardinality on every scrape.
type QueryHashMetricsCollector struct {
	registry *QueryHashRegistry
	serverID string
	desc     *prometheus.Desc
}

func NewQueryHashMetricsCollector(registry *QueryHashRegistry, serverID string) *QueryHashMetricsCollector {
	return &QueryHashMetricsCollector{
		registry: registry,
		serverID: serverID,
		desc: prometheus.NewDesc(
			prometheus.BuildFQName("database_observability", "", "query_hash_info"),
			"Mapping of PostgreSQL queryid to semantic query fingerprint",
			[]string{"queryid", "query_fingerprint", "server_id", "datname"},
			nil,
		),
	}
}

func (c *QueryHashMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *QueryHashMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	for queryID, info := range c.registry.Snapshot() {
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			1,
			queryID,
			info.Fingerprint,
			c.serverID,
			info.DatabaseName,
		)
	}
}
