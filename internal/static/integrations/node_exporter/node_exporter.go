//go:build !windows

package node_exporter

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sort"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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

// nodeExporterVersion holds the version information for the node_exporter library.
// This should match the version of github.com/prometheus/node_exporter being used.
const (
	nodeExporterVersion  = "v1.9.1"
	nodeExporterRevision = "318b01780c89" // From grafana/node_exporter fork
	nodeExporterBranch   = "HEAD"
)

// buildInfoCollector is a custom collector that exposes node_exporter version information.
type buildInfoCollector struct {
	desc *prometheus.Desc
}

func newBuildInfoCollector() *buildInfoCollector {
	return &buildInfoCollector{
		desc: prometheus.NewDesc(
			"node_exporter_build_info",
			"A metric with a constant '1' value labeled by version, revision, branch, goversion from which node_exporter was built, and the goos and goarch for the build.",
			nil,
			prometheus.Labels{
				"version":   nodeExporterVersion,
				"revision":  nodeExporterRevision,
				"branch":    nodeExporterBranch,
				"goversion": runtime.Version(),
				"goos":      runtime.GOOS,
				"goarch":    runtime.GOARCH,
			},
		),
	}
}

func (c *buildInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *buildInfoCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1)
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

	// Register node_exporter_build_info metrics with correct node_exporter version,
	// generally useful for dashboards that depend on them for discovering targets.
	if err := r.Register(newBuildInfoCollector()); err != nil {
		return nil, fmt.Errorf("couldn't register node_exporter build info: %w", err)
	}

	handler := promhttp.HandlerFor(
		prometheus.Gatherers{i.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorHandling:       promhttp.ContinueOnError,
			MaxRequestsInFlight: 0,
			Registry:            i.exporterMetricsRegistry,
		},
	)

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
