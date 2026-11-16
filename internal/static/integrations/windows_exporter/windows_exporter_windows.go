package windows_exporter

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/prometheus-community/windows_exporter/pkg/collector"
)

// New creates a new windows_exporter integration.
func New(logger *slog.Logger, c *Config) (integrations.Integration, error) {
	winExporterConfig, err := c.ToWindowsExporterConfig()
	if err != nil {
		return nil, err
	}
	enabledCollectorNames, _ := normalizeCollectors(c.EnabledCollectors)

	winCol := collector.NewWithConfig(winExporterConfig)
	if err := winCol.Enable(enabledCollectorNames); err != nil {
		return nil, err
	}
	logger.Info("enabled windows_exporter collectors", "collectors", strings.Join(enabledCollectorNames, ","))

	err = winCol.Build(context.Background(), logger)
	if err != nil {
		// Release the resources of the collectors that already started.
		_ = winCol.Close()
		return nil, err
	}

	// Hard-coded 4m timeout to represent the time a series goes stale.
	// TODO: Make configurable if useful.
	handler, err := winCol.NewHandler(4*time.Minute, logger, enabledCollectorNames)
	if err != nil {
		_ = winCol.Close()
		return nil, err
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithLogger(logger),
		integrations.WithCollectors(handler),
		integrations.WithRunner(func(ctx context.Context) error {
			<-ctx.Done()

			// Stop the collector
			err := winCol.Close()

			return errors.Join(ctx.Err(), err)
		}),
	), nil
}
