package integrations

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grafana/agent/internal/build"
	"github.com/grafana/agent/internal/static/integrations/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CollectorIntegration is an integration exposing metrics from one or more Prometheus collectors.
type CollectorIntegration struct {
	name                   string
	cs                     []prometheus.Collector
	includeExporterMetrics bool
	runner                 func(context.Context) error
}

// NewCollectorIntegration creates a basic integration that exposes metrics from multiple prometheus.Collector.
func NewCollectorIntegration(name string, configs ...CollectorIntegrationConfig) *CollectorIntegration {
	i := &CollectorIntegration{
		name: name,
		runner: func(ctx context.Context) error {
			// We don't need to do anything by default, so we can just wait for the context to finish.
			<-ctx.Done()
			return ctx.Err()
		},
	}
	for _, configure := range configs {
		configure(i)
	}
	return i
}

// CollectorIntegrationConfig defines constructor configuration for NewCollectorIntegration
type CollectorIntegrationConfig func(integration *CollectorIntegration)

// WithCollectors adds more collectors to the CollectorIntegration being created.
func WithCollectors(cs ...prometheus.Collector) CollectorIntegrationConfig {
	return func(i *CollectorIntegration) {
		i.cs = append(i.cs, cs...)
	}
}

// WithRunner replaces the runner of the CollectorIntegration.
// The runner function should run while the context provided is not done.
func WithRunner(runner func(context.Context) error) CollectorIntegrationConfig {
	return func(i *CollectorIntegration) {
		i.runner = runner
	}
}

// WithExporterMetricsIncluded can enable the exporter metrics if the flag provided is enabled.
func WithExporterMetricsIncluded(included bool) CollectorIntegrationConfig {
	return func(i *CollectorIntegration) {
		i.includeExporterMetrics = included
	}
}

// MetricsHandler returns the HTTP handler for the integration.
func (i *CollectorIntegration) MetricsHandler() (http.Handler, error) {
	r := prometheus.NewRegistry()
	for _, c := range i.cs {
		if err := r.Register(c); err != nil {
			return nil, fmt.Errorf("couldn't register %s: %w", i.name, err)
		}
	}

	// Register <integration name>_build_info metrics, generally useful for
	// dashboards that depend on them for discovering targets.
	if err := r.Register(build.NewCollector(i.name)); err != nil {
		return nil, fmt.Errorf("couldn't register %s: %w", i.name, err)
	}

	handler := promhttp.HandlerFor(
		r,
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	)

	if i.includeExporterMetrics {
		// Note that we have to use reg here to use the same promhttp metrics for
		// all expositions.
		handler = promhttp.InstrumentMetricHandler(r, handler)
	}

	return handler, nil
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *CollectorIntegration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     i.name,
		MetricsPath: "/metrics",
	}}
}

// Run satisfies Integration.Run.
func (i *CollectorIntegration) Run(ctx context.Context) error {
	return i.runner(ctx)
}
