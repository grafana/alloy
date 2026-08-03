// Package metricsutil holds small Prometheus helpers with no dependencies
// beyond the Prometheus client. Packages that cannot import internal/util,
// for example because internal/util already imports them, can still use
// these helpers by depending on this leaf package instead.
package metricsutil

import "github.com/prometheus/client_golang/prometheus"

// MustRegisterOrReturnExisting registers c on reg. If c is already
// registered, for example because multiple callers share one registerer, it
// returns the existing collector instead of panicking.
// If registration fails for any other reason, it panics.
func MustRegisterOrReturnExisting(reg prometheus.Registerer, c prometheus.Collector) prometheus.Collector {
	if err := reg.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
		panic(err)
	}
	return nil
}
