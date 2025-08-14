package windows_exporter

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/prometheus-community/windows_exporter/pkg/collector"
)

// New creates a new windows_exporter integration.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	// Filter down to the enabled collectors
	enabledCollectorNames := enabledCollectors(c.EnabledCollectors)
	winExporterConfig, err := c.ToWindowsExporterConfig()
	if err != nil {
		return nil, err
	}

	winCol := collector.NewWithConfig(winExporterConfig)
	winCol.Enable(enabledCollectorNames)
	sort.Strings(enabledCollectorNames)
	level.Info(logger).Log("msg", "enabled windows_exporter collectors", "collectors", strings.Join(enabledCollectorNames, ","))

	slogLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	err = winCol.Build(context.Background(), slogLogger)
	if err != nil {
		return nil, err
	}

	// Hard-coded 4m timeout to represent the time a series goes stale.
	// TODO: Make configurable if useful.
	handler, err := winCol.NewHandler(4*time.Minute, slogLogger, enabledCollectorNames)
	if err != nil {
		return nil, err
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(handler),
		integrations.WithRunner(func(ctx context.Context) error {
			<-ctx.Done()

			// Stop the collector
			err := winCol.Close()

			return errors.Join(ctx.Err(), err)
		}),
	), nil
}

func enabledCollectors(input string) []string {
	separated := strings.Split(input, ",")
	unique := map[string]struct{}{}
	for _, s := range separated {
		s = strings.TrimSpace(s)
		if s != "" {
			unique[s] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for s := range unique {
		result = append(result, s)
	}
	return result
}
