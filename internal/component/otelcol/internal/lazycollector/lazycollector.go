package lazycollector

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Collector is a lazy Prometheus collector. The zero value is ready for use.
type Collector struct {
	mut   sync.RWMutex
	inner prometheus.Collector
}

var _ prometheus.Collector = (*Collector)(nil)

// New returns a new lazy Prometheus collector.
func New() *Collector { return &Collector{} }

// Describe implements prometheus.Collector. Because Collector is a
// lazycollector, it is unchecked, so Describe is a no-op.
func (c *Collector) Describe(chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector. If the inner collector is set, its
// Collect method is called. Otherwise, Collect is a no-op.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.inner != nil {
		c.inner.Collect(ch)
	}
}

// Set updates the inner collector used by the lazy collector.
func (c *Collector) Set(inner prometheus.Collector) {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.inner = inner
}
