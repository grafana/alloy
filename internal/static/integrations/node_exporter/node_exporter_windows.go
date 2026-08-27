package node_exporter

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/grafana/alloy/internal/static/integrations/config"
)

// Integration is the node_exporter integration. On Windows platforms,
// this integration does nothing and will print a warning if enabled.
type Integration struct {
}

// New creates a fake node_exporter integration.
func New(logger *slog.Logger, _ *Config) (*Integration, error) {
	logger.Warn("the node_exporter does not work on Windows; enabling it otherwise will do nothing")
	return &Integration{}, nil
}

// MetricsHandler satisfies Integration.RegisterRoutes.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	return http.NotFoundHandler(), nil
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	// No-op: nothing to scrape.
	return []config.ScrapeConfig{}
}

// Run satisfies Integration.Run.
func (i *Integration) Run(ctx context.Context) error {
	// We don't need to do anything here, so we can just wait for the context to
	// finish.
	<-ctx.Done()
	return ctx.Err()
}
