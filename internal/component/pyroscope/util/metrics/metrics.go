package metrics

import "github.com/prometheus/client_golang/prometheus"

// MustRegisterOrGet is a copy of util.MustRegisterOrGet but does not bring hundreds of transitive dependencies
// A little copying is better than a little dependency.
// https://www.youtube.com/watch?v=PAAkCSZUG1c&t=568s
// The Previous attempt to fix this globally stalled: https://github.com/grafana/alloy/pull/4369
// So for now it is in the pyroscope subpackage
func MustRegisterOrGet(reg prometheus.Registerer, c prometheus.Collector) prometheus.Collector {
	if err := reg.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
		panic(err)
	}
	return c
}
