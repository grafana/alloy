// Package agent is an "example" integration that has very little functionality,
// but is still useful in practice. The Agent integration re-exposes the Agent's
// own metrics endpoint and allows the Agent to scrape itself.
package agent

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
)

// Config controls the Agent integration.
type Config struct{}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "agent"
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

// NewIntegration converts this config into an instance of an integration.
func (c *Config) NewIntegration(l *slog.Logger) (integrations.Integration, error) {
	return New(l, c), nil
}

func init() {
	integrations.RegisterIntegration(&Config{})
}

// Integration is the Agent integration. The Agent integration scrapes the
// Agent's own metrics.
type Integration struct {
	c   *Config
	log *slog.Logger
}

// New creates a new Agent integration.
func New(log *slog.Logger, c *Config) *Integration {
	return &Integration{c: c, log: log}
}

// MetricsHandler satisfies Integration.RegisterRoutes.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	// This is promhttp.Handler() with an ErrorLog added. Keep the
	// InstrumentMetricHandler wrapper so promhttp_metric_handler_requests_total
	// and promhttp_metric_handler_requests_in_flight stay exposed.
	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog: util.PromHTTPErrorLogger(i.log),
	})
	return promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, handler), nil
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     i.c.Name(),
		MetricsPath: "/metrics",
	}}
}

// Run satisfies Integration.Run.
func (i *Integration) Run(ctx context.Context) error {
	// We don't need to do anything here, so we can just wait for the context to
	// finish.
	<-ctx.Done()
	return ctx.Err()
}
