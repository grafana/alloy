//go:build !linux

package cadvisor

import (
	"context"
	"net/http"

	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// NewIntegration creates a new cadvisor integration
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	level.Warn(logger).Log("msg", "the cadvisor integration only works on linux; enabling it on other platforms will do nothing")
	return &stubIntegration{}, nil
}

// stubIntegration implements a no-op integration for use on platforms not supported by an integration
type stubIntegration struct{}

// MetricsHandler returns an http.NotFoundHandler to satisfy the Integration interface
func (i *stubIntegration) MetricsHandler() (http.Handler, error) {
	return http.NotFoundHandler(), nil
}

// ScrapeConfigs returns an empty list of scrape configs, since there is nothing to scrape
func (i *stubIntegration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{}
}

// Run just waits for the context to finish
func (i *stubIntegration) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
