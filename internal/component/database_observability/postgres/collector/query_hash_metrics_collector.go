package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// QueryHashMetricsCollector exposes queryid to queryhash mappings as Prometheus metrics
type QueryHashMetricsCollector struct {
	registry *QueryHashRegistry
	serverID string
	desc     *prometheus.Desc
}

// NewQueryHashMetricsCollector creates a new QueryHashMetricsCollector
func NewQueryHashMetricsCollector(registry *QueryHashRegistry, serverID string) *QueryHashMetricsCollector {
	return &QueryHashMetricsCollector{
		registry: registry,
		serverID: serverID,
		desc: prometheus.NewDesc(
			prometheus.BuildFQName("database_observability", "", "query_hash_info"),
			"Mapping of PostgreSQL queryid to internal queryhash",
			[]string{"queryid", "queryhash", "server_id", "datname"},
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *QueryHashMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

// Collect implements prometheus.Collector
func (c *QueryHashMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	mappings := c.registry.GetAll()

	for queryID, info := range mappings {
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			1,
			queryID,
			info.QueryHash,
			c.serverID,
			info.DatabaseName,
		)
	}
}
