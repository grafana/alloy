package collector

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// cachedMetrics holds the metrics produced by the last successful run of a
// query, so that a scrape can serve them without querying the database.
type cachedMetrics struct {
	mu      sync.RWMutex
	metrics []prometheus.Metric
}

// set replaces the cached metrics.
func (c *cachedMetrics) set(metrics []prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = metrics
}

// emit sends the cached metrics to ch.
func (c *cachedMetrics) emit(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, m := range c.metrics {
		ch <- m
	}
}
