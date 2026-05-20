//go:build cgo

package oracledb_exporter

import (
	"errors"
	"log/slog"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	oe "github.com/oracle/oracle-db-appdev-monitoring/collector"
	"k8s.io/utils/ptr"
)

// New creates a new oracledb integration. The integration scrapes metrics
// from an OracleDB exporter running with the https://github.com/oracle/oracle-db-appdev-monitoring
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	slogLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	targets, err := c.normalizedTargets()
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, errors.New("no databases configured")
	}

	databases := make(map[string]oe.DatabaseConfig, len(targets))
	for _, t := range targets {
		databases[t.name] = oe.DatabaseConfig{
			URL:      t.url,
			Username: t.user,
			Password: t.pass,
			ConnectConfig: oe.ConnectConfig{
				MaxIdleConns: ptr.To(c.MaxIdleConns),
				MaxOpenConns: ptr.To(c.MaxOpenConns),
				QueryTimeout: ptr.To(c.QueryTimeout),
			},
			Labels: t.labels,
		}
	}

	// ScrapeInterval must be non-nil: the collector's Collect path dereferences it.
	scrapeInterval := time.Duration(0)
	oeExporter := oe.NewExporter(slogLogger, &oe.MetricsConfiguration{
		Databases: databases,
		Metrics: oe.MetricsFilesConfig{
			Custom:         c.CustomMetrics,
			Default:        c.DefaultMetrics,
			ScrapeInterval: &scrapeInterval,
		},
	})

	if oeExporter == nil {
		return nil, errors.New("failed to create oracledb exporter")
	}
	// The upstream binary runs InitializeDatabases on startup to warm up the pool
	// and flip StartupReady; without it, Collect only logs "Database connection in progress"
	// and never scrapes Oracle metrics (see oracle-db-appdev-monitoring collector.Database).
	go oeExporter.InitializeDatabases()
	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(oeExporter)), nil
}
