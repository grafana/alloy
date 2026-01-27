package collector

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
)

// QueryHashInfo contains information about a query hash
type QueryHashInfo struct {
	QueryHash    string
	DatabaseName string
	LastSeen     time.Time
}

// QueryHashRegistry maintains a mapping of queryid to queryhash
type QueryHashRegistry struct {
	mu    sync.RWMutex
	cache *expirable.LRU[string, QueryHashInfo]
}

// NewQueryHashRegistry creates a new QueryHashRegistry
func NewQueryHashRegistry(size int, ttl time.Duration) *QueryHashRegistry {
	return &QueryHashRegistry{
		cache: expirable.NewLRU[string, QueryHashInfo](size, nil, ttl),
	}
}

// Set stores a queryid to queryhash mapping
func (r *QueryHashRegistry) Set(queryID, queryHash, databaseName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache.Add(queryID, QueryHashInfo{
		QueryHash:    queryHash,
		DatabaseName: databaseName,
		LastSeen:     time.Now(),
	})
}

// Get retrieves the QueryHashInfo for a given queryID
func (r *QueryHashRegistry) Get(queryID string) (QueryHashInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.cache.Get(queryID)
	return info, ok
}

// GetAll returns all current queryid to queryhash mappings
func (r *QueryHashRegistry) GetAll() map[string]QueryHashInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]QueryHashInfo)
	for _, key := range r.cache.Keys() {
		if info, ok := r.cache.Peek(key); ok {
			result[key] = info
		}
	}
	return result
}

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
