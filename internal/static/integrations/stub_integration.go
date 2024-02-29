package integrations

import (
	"context"
	"net/http"

	"github.com/grafana/agent/internal/static/integrations/config"
)

// StubIntegration implements a no-op integration for use on platforms not supported by an integration
type StubIntegration struct{}

// MetricsHandler returns an http.NotFoundHandler to satisfy the Integration interface
func (i *StubIntegration) MetricsHandler() (http.Handler, error) {
	return http.NotFoundHandler(), nil
}

// ScrapeConfigs returns an empty list of scrape configs, since there is nothing to scrape
func (i *StubIntegration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{}
}

// Run just waits for the context to finish
func (i *StubIntegration) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
