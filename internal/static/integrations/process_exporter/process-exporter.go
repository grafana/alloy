//go:build !linux

// Package process_exporter embeds https://github.com/ncabatoff/process-exporter
package process_exporter

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/grafana/alloy/internal/static/integrations/config"
)

// Integration is the process_exporter integration. On non-Linux platforms,
// this integrats nothing and will print a warning if enabled.
type Integration struct {
	c *Config
}

// New creates a process_exporter integration for non-Linux platforms, which is always a
// no-op.
func New(logger *slog.Logger, c *Config) (*Integration, error) {
	logger.Warn("the process_exporter only works on Linux; enabling it otherwise will do nothing")
	return &Integration{c: c}, nil
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
