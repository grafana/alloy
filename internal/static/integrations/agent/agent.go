// Package agent is an "example" integration that has very little functionality,
// but is still useful in practice. The Agent integration re-exposes the Agent's
// own metrics endpoint and allows the Agent to scrape itself.
package agent

import (
	"context"
	"net/http"

	"github.com/go-kit/log"
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
func (c *Config) NewIntegration(_ log.Logger) (integrations.Integration, error) {
	return New(c), nil
}

func init() {
	integrations.RegisterIntegration(&Config{})
}

// Integration is the Agent integration. The Agent integration scrapes the
// Agent's own metrics.
type Integration struct {
	c *Config
}

// New creates a new Agent integration.
func New(c *Config) *Integration {
	return &Integration{c: c}
}

// MetricsHandler satisfies Integration.RegisterRoutes.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	return promhttp.Handler(), nil
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
