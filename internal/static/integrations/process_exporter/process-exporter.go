//go:build !linux

// Package process_exporter embeds https://github.com/ncabatoff/process-exporter
package process_exporter

import (
	"context"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/static/integrations/config"
)

// Integration is the process_exporter integration. On non-Linux platforms,
// this integration does nothing and will print a warning if enabled.
type Integration struct {
	c *Config
}

// New creates a process_exporter integration for non-Linux platforms, which is always a
// no-op.
func New(logger log.Logger, c *Config) (*Integration, error) {
	level.Warn(logger).Log("msg", "the process_exporter only works on Linux; enabling it otherwise will do nothing")
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
