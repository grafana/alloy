package testtarget

import (
	"net/http/httptest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/component/discovery"
)

func NewTestTarget() *TestTarget {
	// Start server to expose prom metrics
	registry := prometheus.NewRegistry()
	srv := httptest.NewServer(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return &TestTarget{
		server:   srv,
		registry: registry,
	}
}

// TestTarget is a test target for prometheus metrics that exposes the metrics from provided registry via HTTP.
// It must be closed after use using Close method.
type TestTarget struct {
	server   *httptest.Server
	registry *prometheus.Registry
}

func (t *TestTarget) AddCounter(opts prometheus.CounterOpts) prometheus.Counter {
	counter := prometheus.NewCounter(opts)
	t.registry.MustRegister(counter)
	return counter
}

func (t *TestTarget) AddGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	gauge := prometheus.NewGauge(opts)
	t.registry.MustRegister(gauge)
	return gauge
}

func (t *TestTarget) AddHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	histogram := prometheus.NewHistogram(opts)
	t.registry.MustRegister(histogram)
	return histogram
}

func (t *TestTarget) Target() discovery.Target {
	return discovery.NewTargetFromMap(map[string]string{
		"__address__": t.server.Listener.Addr().String(),
	})
}

func (t *TestTarget) Registry() *prometheus.Registry {
	return t.registry
}

func (t *TestTarget) URL() string {
	return t.server.URL
}

func (t *TestTarget) Close() {
	t.server.Close()
}
