package collector

import (
	"slices"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// cachedDatabaseMetrics holds, per database, the metrics produced by the last
// successful run against it, so that a scrape can serve them without querying
// any database. A database that fails in a later run keeps its previous
// results.
type cachedDatabaseMetrics struct {
	mu      sync.RWMutex
	metrics map[string][]prometheus.Metric
}

// set replaces the cached metrics of one database.
func (c *cachedDatabaseMetrics) set(database string, metrics []prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metrics == nil {
		c.metrics = make(map[string][]prometheus.Metric)
	}
	c.metrics[database] = metrics
}

// keepOnly drops the cached metrics of every database not in databases.
func (c *cachedDatabaseMetrics) keepOnly(databases []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for database := range c.metrics {
		if !slices.Contains(databases, database) {
			delete(c.metrics, database)
		}
	}
}

// emit sends the cached metrics of every database to ch.
func (c *cachedDatabaseMetrics) emit(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, metrics := range c.metrics {
		for _, m := range metrics {
			ch <- m
		}
	}
}
