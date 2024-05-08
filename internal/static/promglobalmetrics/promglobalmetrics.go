// Package promglobalmetrics is used by static mode to create and register prometheus metrics for service discovery.
// In the past, Prometheus SD metrics were registered with the global registry in the Prometheus codebase.
// This is no longer the case. Alloy uses new Prometheus features to register metrics per component,
// which enables it to show more accurate SD metrics because each SD instance will have its own series.
// For static mode code paths (or whatever is left of them) we keep simulating the old behavior of using global metrics.
package promglobalmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/discovery"
)

var PromSdMetrics map[string]discovery.DiscovererMetrics

var PromDiscoveryManagerRegistry RecyclingRegistry
var PromScrapeManagerRegistry RecyclingRegistry

func init() {
	var err error
	PromSdMetrics, err = discovery.CreateAndRegisterSDMetrics(prometheus.DefaultRegisterer)
	if err != nil {
		panic(err)
	}

	PromDiscoveryManagerRegistry = NewRecyclingRegistry(prometheus.DefaultRegisterer)
	PromScrapeManagerRegistry = NewRecyclingRegistry(prometheus.DefaultRegisterer)
}

// RecyclingRegistry will never throw an AlreadyRegistered error.
// It's useful when you want to reuse the same metrics.
type RecyclingRegistry struct {
	reg prometheus.Registerer
}

var _ prometheus.Registerer = RecyclingRegistry{}

func NewRecyclingRegistry(reg prometheus.Registerer) RecyclingRegistry {
	return RecyclingRegistry{reg: reg}
}

// MustRegister implements prometheus.Registerer.
func (r RecyclingRegistry) MustRegister(cols ...prometheus.Collector) {
	for _, c := range cols {
		err := r.Register(c)
		if err != nil {
			panic(err)
		}
	}
}

// Register implements prometheus.Registerer.
func (r RecyclingRegistry) Register(c prometheus.Collector) error {
	err := r.reg.Register(c)
	if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
		return nil
	} else {
		return err
	}
}

// Unregister implements prometheus.Registerer.
func (r RecyclingRegistry) Unregister(c prometheus.Collector) bool {
	return r.reg.Unregister(c)
}
