package gcp_exporter

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/multierror"
	"github.com/prometheus-community/stackdriver_exporter/collectors"
	"github.com/prometheus-community/stackdriver_exporter/delta"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("gcp"))
}

type Config struct {
	ProjectIDs            []string      `yaml:"project_ids"`
	MetricPrefixes        []string      `yaml:"metrics_prefixes"`
	ExtraFilters          []string      `yaml:"extra_filters"`
	RequestInterval       time.Duration `yaml:"request_interval"`
	RequestOffset         time.Duration `yaml:"request_offset"`
	IngestDelay           bool          `yaml:"ingest_delay"`
	DropDelegatedProjects bool          `yaml:"drop_delegated_projects"`
	ClientTimeout         time.Duration `yaml:"gcp_client_timeout"`
}

var DefaultConfig = Config{
	ClientTimeout:         15 * time.Second,
	RequestInterval:       5 * time.Minute,
	RequestOffset:         0,
	IngestDelay:           false,
	DropDelegatedProjects: false,
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

func (c *Config) Name() string {
	return "gcp_exporter"
}

func (c *Config) InstanceKey(_ string) (string, error) {
	// We use a hash of the config so our key is unique when leveraged with v2
	// The config itself doesn't have anything which can uniquely identify it.
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:]), nil
}

func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	svc, err := createMonitoringService(context.Background(), c.ClientTimeout)
	if err != nil {
		return nil, err
	}

	logger := slog.New(logging.NewSlogGoKitHandler(l))

	var gcpCollectors []prometheus.Collector
	var counterStores []*SelfPruningDeltaStore[collectors.ConstMetric]
	var histogramStores []*SelfPruningDeltaStore[collectors.HistogramMetric]
	for _, projectID := range c.ProjectIDs {
		counterStore := NewSelfPruningDeltaStore[collectors.ConstMetric](l, delta.NewInMemoryCounterStore(logger, 30*time.Minute))
		histogramStore := NewSelfPruningDeltaStore[collectors.HistogramMetric](l, delta.NewInMemoryHistogramStore(logger, 30*time.Minute))
		monitoringCollector, err := collectors.NewMonitoringCollector(
			projectID,
			svc,
			collectors.MonitoringCollectorOptions{
				MetricTypePrefixes:    c.MetricPrefixes,
				ExtraFilters:          parseMetricExtraFilters(c.ExtraFilters),
				RequestInterval:       c.RequestInterval,
				RequestOffset:         c.RequestOffset,
				IngestDelay:           c.IngestDelay,
				DropDelegatedProjects: c.DropDelegatedProjects,

				// If FillMissingLabels ensures all metrics with the same name have the same label set. It's not often
				// that GCP metrics have different labels but if it happens the prom registry will panic.
				FillMissingLabels: true,

				// If AggregateDeltas is disabled the data produced is not useful at all. See https://github.com/prometheus-community/stackdriver_exporter#what-to-know-about-aggregating-delta-metrics
				// for more info
				AggregateDeltas: true,
			},
			logger,
			counterStore,
			histogramStore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create monitoring collector: %w", err)
		}
		counterStores = append(counterStores, counterStore)
		histogramStores = append(histogramStores, histogramStore)
		gcpCollectors = append(gcpCollectors, monitoringCollector)
	}

	run := func(ctx context.Context) error {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				level.Debug(l).Log("msg", "Starting delta store pruning", "number_of_stores", len(counterStores)+len(histogramStores))
				for _, store := range counterStores {
					store.Prune(ctx)
				}
				for _, store := range histogramStores {
					store.Prune(ctx)
				}
				level.Debug(l).Log("msg", "Finished delta store pruning", "number_of_stores", len(counterStores)+len(histogramStores))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return integrations.NewCollectorIntegration(
		c.Name(), integrations.WithCollectors(gcpCollectors...), integrations.WithRunner(run),
	), nil
}

func (c *Config) Validate() error {
	configErrors := multierror.MultiError{}

	if len(c.ProjectIDs) == 0 {
		configErrors.Add(errors.New("no project_ids defined"))
	}

	if len(c.MetricPrefixes) == 0 {
		configErrors.Add(errors.New("at least 1 metrics_prefixes is required"))
	}

	if len(c.ExtraFilters) > 0 {
		filterPrefixToFilter := map[string][]string{}
		for _, filter := range c.ExtraFilters {
			splitFilter := strings.SplitN(filter, ":", 2)
			if len(splitFilter) <= 1 {
				configErrors.Add(fmt.Errorf("%s is an invalid filter a filter must be of the form <metric_type>:<filter_expression>", filter))
				continue
			}
			filterPrefix := splitFilter[0]
			if _, exists := filterPrefixToFilter[filterPrefix]; !exists {
				filterPrefixToFilter[filterPrefix] = []string{}
			}
			filterPrefixToFilter[filterPrefix] = append(filterPrefixToFilter[filterPrefix], filter)
		}

		for filterPrefix, filters := range filterPrefixToFilter {
			validFilterPrefix := false
			for _, metricPrefix := range c.MetricPrefixes {
				if strings.HasPrefix(metricPrefix, filterPrefix) {
					validFilterPrefix = true
					break
				}
			}
			if !validFilterPrefix {
				configErrors.Add(fmt.Errorf("no metric_prefixes started with %s which means the extra_filters %s will not have any effect", filterPrefix, strings.Join(filters, ",")))
			}
		}
	}

	return configErrors.Err()
}

func createMonitoringService(ctx context.Context, httpTimeout time.Duration) (*monitoring.Service, error) {
	googleClient, err := google.DefaultClient(ctx, monitoring.MonitoringReadScope)
	if err != nil {
		return nil, fmt.Errorf("error creating Google client: %v", err)
	}

	googleClient.Timeout = httpTimeout
	googleClient.Transport = rehttp.NewTransport(
		googleClient.Transport,
		rehttp.RetryAll(
			rehttp.RetryMaxRetries(4),
			rehttp.RetryStatuses(http.StatusServiceUnavailable)),
		rehttp.ExpJitterDelay(time.Second, 5*time.Second),
	)

	monitoringService, err := monitoring.NewService(ctx,
		option.WithHTTPClient(googleClient),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Google Stackdriver Monitoring service: %v", err)
	}

	return monitoringService, nil
}

func parseMetricExtraFilters(filters []string) []collectors.MetricFilter {
	var extraFilters []collectors.MetricFilter
	for _, ef := range filters {
		splitFilter := strings.SplitN(ef, ":", 2)
		if len(splitFilter) == 2 && splitFilter[0] != "" {
			extraFilter := collectors.MetricFilter{
				TargetedMetricPrefix: splitFilter[0],
				FilterQuery:          splitFilter[1],
			}
			extraFilters = append(extraFilters, extraFilter)
		}
	}
	return extraFilters
}
