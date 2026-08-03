package util

import "github.com/prometheus/client_golang/prometheus"

// MustRegisterOrGet will attempt to register the supplied collector into the register. If it's already
// registered, it will return that one.
// In case that the register procedure fails with something other than already registered, this will panic.
func MustRegisterOrGet(reg prometheus.Registerer, c prometheus.Collector) prometheus.Collector {
	if err := reg.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
		panic(err)
	}
	return c
}
