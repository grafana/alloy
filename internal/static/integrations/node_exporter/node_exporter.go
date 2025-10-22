//go:build !windows

package node_exporter

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/node_exporter/collector"
)

// Integration is the node_exporter integration. The integration scrapes metrics
// from the host Linux-based system.
type Integration struct {
	c      *Config
	logger log.Logger
	nc     *collector.NodeCollector

	exporterMetricsRegistry *prometheus.Registry
}

// New creates a new node_exporter integration.
func New(log log.Logger, c *Config) (*Integration, error) {
	cfg := c.mapConfigToNodeConfig()
	nc, err := collector.NewNodeCollector(cfg, slog.New(logging.NewSlogGoKitHandler(log)))
	if err != nil {
		return nil, fmt.Errorf("failed to create node_exporter: %w", err)
	}

	level.Info(log).Log("msg", "Enabled node_exporter collectors")
	collectors := []string{}
	for n := range nc.Collectors {
		collectors = append(collectors, n)
	}
	sort.Strings(collectors)
	for _, c := range collectors {
		level.Info(log).Log("collector", c)
	}

	return &Integration{
		c:      c,
		logger: log,
		nc:     nc,

		exporterMetricsRegistry: prometheus.NewRegistry(),
	}, nil
}

// MetricsHandler implements Integration.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	r := prometheus.NewRegistry()
	if err := r.Register(i.nc); err != nil {
		return nil, fmt.Errorf("couldn't register node_exporter node collector: %w", err)
	}
	handler := promhttp.HandlerFor(
		prometheus.Gatherers{i.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorHandling:       promhttp.ContinueOnError,
			MaxRequestsInFlight: 0,
			Registry:            i.exporterMetricsRegistry,
		},
	)

	// Register node_exporter_build_info metrics, generally useful for
	// dashboards that depend on them for discovering targets.
	if err := r.Register(build.NewCollector(i.c.Name())); err != nil {
		return nil, fmt.Errorf("couldn't register %s: %w", i.c.Name(), err)
	}

	if i.c.IncludeExporterMetrics {
		// Note that we have to use reg here to use the same promhttp metrics for
		// all expositions.
		handler = promhttp.InstrumentMetricHandler(i.exporterMetricsRegistry, handler)
	}

	return handler, nil
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
