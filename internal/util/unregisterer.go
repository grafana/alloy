package util

import "github.com/prometheus/client_golang/prometheus"

// Unregisterer is a Prometheus Registerer that can unregister all collectors
// passed to it.
type Unregisterer interface {
	prometheus.Registerer
	UnregisterAll() bool
}

// WrapWithUnregisterer wraps a prometheus Registerer with capabilities to
// unregister all collectors.
func WrapWithUnregisterer(reg prometheus.Registerer) Unregisterer {
	return &unregisterer{
		wrap: reg,
		cs:   make(map[prometheus.Collector]struct{}),
	}
}

type unregisterer struct {
	wrap prometheus.Registerer
	cs   map[prometheus.Collector]struct{}
}

// Register implements prometheus.Registerer.
func (u *unregisterer) Register(c prometheus.Collector) error {
	if u.wrap == nil {
		return nil
	}

	err := u.wrap.Register(c)
	if err != nil {
		return err
	}
	u.cs[c] = struct{}{}
	return nil
}

// MustRegister implements prometheus.Registerer.
func (u *unregisterer) MustRegister(cs ...prometheus.Collector) {
	for _, c := range cs {
		if err := u.Register(c); err != nil {
			panic(err)
		}
	}
}

// Unregister implements prometheus.Registerer.
func (u *unregisterer) Unregister(c prometheus.Collector) bool {
	if u.wrap != nil && u.wrap.Unregister(c) {
		delete(u.cs, c)
		return true
	}
	return false
}

// UnregisterAll unregisters all collectors that were registered through the
// Registerer.
func (u *unregisterer) UnregisterAll() bool {
	success := true
	for c := range u.cs {
		if !u.Unregister(c) {
			success = false
		}
	}
	return success
}
