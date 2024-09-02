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

// An "unchecked collector" is a collector which returns an empty description.
// It is described in the Prometheus documentation, here:
// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#hdr-Custom_Collectors_and_constant_Metrics
//
// > Alternatively, you could return no Desc at all, which will mark the Collector “unchecked”.
// > No checks are performed at registration time, but metric consistency will still be ensured at scrape time,
// > i.e. any inconsistencies will lead to scrape errors. Thus, with unchecked Collectors,
// > the responsibility to not collect metrics that lead to inconsistencies in the total scrape result
// > lies with the implementer of the Collector. While this is not a desirable state, it is sometimes necessary.
// > The typical use case is a situation where the exact metrics to be returned by a Collector cannot be predicted
// > at registration time, but the implementer has sufficient knowledge of the whole system to guarantee metric consistency.
//
// Unchecked collectors are used in the Loki "metrics" stage of the Loki "process" component.
//
// The isUncheckedCollector function is similar to how Prometheus' Go client extracts the metric description:
// https://github.com/prometheus/client_golang/blob/45f1e72421d9d11af6be784ad60b7389f7543e70/prometheus/registry.go#L372-L381
func isUncheckedCollector(c prometheus.Collector) bool {
	descChan := make(chan *prometheus.Desc, 10)

	go func() {
		c.Describe(descChan)
		close(descChan)
	}()

	i := 0
	for range descChan {
		i += 1
	}

	return i == 0
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

	if isUncheckedCollector(c) {
		return nil
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
	if isUncheckedCollector(c) {
		return true
	}

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
