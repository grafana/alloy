// Package process_exporter embeds https://github.com/ncabatoff/process-exporter
package process_exporter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ncabatoff/process-exporter/collector"
)

// Integration is the process_exporter integration. The integration scrapes
// metrics based on information in the /proc filesystem for Linux.
// Agent's own metrics.
type Integration struct {
	c         *Config
	collector *collector.NamedProcessCollector
}

// New creates a new instance of the process_exporter integration.
func New(logger log.Logger, c *Config) (*Integration, error) {
	cfg, err := c.ProcessExporter.ToConfig()
	if err != nil {
		return nil, fmt.Errorf("process_names is invalid: %w", err)
	}

	pc, err := collector.NewProcessCollector(collector.ProcessCollectorOption{
		ProcFSPath:        c.ProcFSPath,
		Children:          c.Children,
		Threads:           c.Threads,
		GatherSMaps:       c.SMaps,
		Namer:             cfg.MatchNamers,
		Recheck:           c.Recheck,
		RemoveEmptyGroups: c.RemoveEmptyGroups,
		Debug:             false,
	})
	if err != nil {
		return nil, err
	}

	return &Integration{c: c, collector: pc}, nil
}

// MetricsHandler satisfies Integration.RegisterRoutes.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	r := prometheus.NewRegistry()
	if err := r.Register(i.collector); err != nil {
		return nil, fmt.Errorf("couldn't register process_exporter collector: %w", err)
	}

	// Register process_exporter_build_info metrics, generally useful for
	// dashboards that depend on them for discovering targets.
	if err := r.Register(build.NewCollector("process_exporter")); err != nil {
		return nil, fmt.Errorf("couldn't register process_exporter: %w", err)
	}

	return promhttp.HandlerFor(
		prometheus.Gatherers{r},
		promhttp.HandlerOpts{
			ErrorHandling:       promhttp.ContinueOnError,
			MaxRequestsInFlight: 0,
		},
	), nil
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
