package windows_exporter //nolint:golint

import (
	"sort"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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

	winCol := collector.NewWithConfig(logger, winExporterConfig)
	winCol.Enable(enabledCollectorNames)
	sort.Strings(enabledCollectorNames)
	level.Info(logger).Log("msg", "enabled windows_exporter collectors", "collectors", strings.Join(enabledCollectorNames, ","))

	err = winCol.Build()
	if err != nil {
		return nil, err
	}
	err = winCol.SetPerfCounterQuery()
	if err != nil {
		return nil, err
	}

	return integrations.NewCollectorIntegration(c.Name(), integrations.WithCollectors(
		// Hard-coded 4m timeout to represent the time a series goes stale.
		// TODO: Make configurable if useful.
		collector.NewPrometheus(4*time.Minute, &winCol, logger),
	)), nil
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
